package hot

import (
	"fmt"
	"sync/atomic"

	"github.com/linxGnu/grocksdb"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/pkg/utils"
)

type Store struct {
	db      *grocksdb.DB
	cfs     map[string]*grocksdb.ColumnFamilyHandle
	wo      *grocksdb.WriteOptions
	ro      *grocksdb.ReadOptions
	writes  atomic.Int64
	dataDir string
}

func openRocksDB(dataDir string, cfg *config.Config) (*Store, error) {
	dbPath := dataDir

	if err := utils.EnsureDir(dbPath); err != nil {
		return nil, fmt.Errorf("failed to create hot tier directory: %w", err)
	}

	dbOpts := newDBOptions()
	cfNames := AllColumnFamilies()
	cfOpts := make([]*grocksdb.Options, len(cfNames))
	for i := range cfNames {
		cfOpts[i] = newColumnFamilyOptions()
	}

	db, cfHandles, err := grocksdb.OpenDbColumnFamilies(dbOpts, dbPath, cfNames, cfOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open RocksDB: %w", err)
	}

	cfs := make(map[string]*grocksdb.ColumnFamilyHandle, len(cfNames))
	for i, name := range cfNames {
		cfs[name] = cfHandles[i]
	}

	return &Store{
		db:      db,
		cfs:     cfs,
		wo:      newWriteOptions(),
		ro:      newReadOptions(),
		dataDir: dbPath,
	}, nil
}

func (s *Store) Put(cf string, key, value []byte) error {
	handle, err := s.cfHandle(cf)
	if err != nil {
		return err
	}
	if err := s.db.PutCF(s.wo, handle, key, value); err != nil {
		return fmt.Errorf("RocksDB put failed in CF %q: %w", cf, err)
	}
	s.writes.Add(1)
	return nil
}

func (s *Store) Get(cf string, key []byte) ([]byte, error) {
	handle, err := s.cfHandle(cf)
	if err != nil {
		return nil, err
	}
	slice, err := s.db.GetCF(s.ro, handle, key)
	if err != nil {
		return nil, fmt.Errorf("RocksDB get failed in CF %q: %w", cf, err)
	}
	defer slice.Free()
	if !slice.Exists() {
		return nil, nil
	}
	data := make([]byte, len(slice.Data()))
	copy(data, slice.Data())
	return data, nil
}

func (s *Store) Delete(cf string, key []byte) error {
	handle, err := s.cfHandle(cf)
	if err != nil {
		return err
	}
	return s.db.DeleteCF(s.wo, handle, key)
}

func (s *Store) Scan(cf string, prefix []byte, fn func(key, value []byte) bool) error {
	handle, err := s.cfHandle(cf)
	if err != nil {
		return err
	}

	ro := newReadOptions()
	defer ro.Destroy()

	it := s.db.NewIteratorCF(ro, handle)
	defer it.Close()

	if len(prefix) > 0 {
		it.Seek(prefix)
	} else {
		it.SeekToFirst()
	}

	for ; it.Valid(); it.Next() {
		k := it.Key()
		v := it.Value()

		keyCopy := make([]byte, len(k.Data()))
		copy(keyCopy, k.Data())
		valCopy := make([]byte, len(v.Data()))
		copy(valCopy, v.Data())

		k.Free()
		v.Free()

		if !fn(keyCopy, valCopy) {
			break
		}
	}

	if err := it.Err(); err != nil {
		return fmt.Errorf("RocksDB scan error in CF %q: %w", cf, err)
	}
	return nil
}

func (s *Store) Close() {
	s.wo.Destroy()
	s.ro.Destroy()
	for _, handle := range s.cfs {
		handle.Destroy()
	}
	s.db.Close()
}

func (s *Store) TotalWrites() int64 {
	return s.writes.Load()
}

func (s *Store) cfHandle(cf string) (*grocksdb.ColumnFamilyHandle, error) {
	handle, ok := s.cfs[cf]
	if !ok {
		return nil, fmt.Errorf("unknown column family: %q", cf)
	}
	return handle, nil
}
