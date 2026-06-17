# Plomvix SQL Store

The `sql/store` package provides an ordered, **in-memory** key-value store
built on `internal/engine/sql/key` for key ordering.

## Design

- **sorted slice** — entries are stored in a single sorted slice, ordered by
  `key.Key.Compare()`
- **sort.Search** — binary search is used for lookup and insertion position
- **sync.RWMutex** — a single read/write mutex protects all access
- **half-open** range scan with **`[start, end)`** semantics
- **copy-safety** — values are copied on Put, Get, and Scan; callers cannot
  mutate store internals

This is an **in-memory** stepping stone, not a persistent storage engine.

## API

```go
func New() *Store
func (s *Store) Put(k key.Key, value []byte) error
func (s *Store) Get(k key.Key) ([]byte, error)
func (s *Store) Delete(k key.Key) error
func (s *Store) Scan(start, end key.Key) ([]Entry, error)
func (s *Store) Len() int
```

## Errors

- `ErrNotFound` — key not present in the store
- `ErrNilStore` — operation attempted on a nil `*Store`

## Non-Goals

The in-memory store intentionally does not implement:

- WAL
- disk persistence
- storage pages
- buffer pool
- **no transactions**
- compaction
- snapshots
- sharded locking
