package hot

import (
	"github.com/linxGnu/grocksdb"
)

func newDBOptions() *grocksdb.Options {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetCreateIfMissingColumnFamilies(true)
	opts.SetWriteBufferSize(64 * 1024 * 1024)
	opts.SetMaxWriteBufferNumber(2)
	opts.IncreaseParallelism(4)
	opts.SetCompression(grocksdb.SnappyCompression)
	return opts
}

func newColumnFamilyOptions() *grocksdb.Options {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCompression(grocksdb.SnappyCompression)
	opts.SetWriteBufferSize(64 * 1024 * 1024)
	return opts
}

func newWriteOptions() *grocksdb.WriteOptions {
	wo := grocksdb.NewDefaultWriteOptions()
	wo.SetSync(false)
	return wo
}

func newReadOptions() *grocksdb.ReadOptions {
	return grocksdb.NewDefaultReadOptions()
}
