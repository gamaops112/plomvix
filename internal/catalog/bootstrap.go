package catalog

import (
	"context"
	"fmt"
	"math"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
)

// bootstrap initializes the system tables and meta on a fresh Heap.
// Must be called without holding mu.
func (c *catalog) bootstrap(ctx context.Context) error {
	// Open system table handles.
	tbl, err := c.h.OpenTable(schemaTables)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_tables: %w", err)
	}
	c.tablesHandle = tbl

	usr, err := c.h.OpenTable(schemaUsers)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_users: %w", err)
	}
	c.usersHandle = usr

	meta, err := c.h.OpenTable(schemaMeta)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_meta: %w", err)
	}
	c.metaHandle = meta

	// Check if meta has the nextTxID key.
	maxTx := heap.Tx{ID: math.MaxUint64}
	_, err = c.metaHandle.Get(ctx, maxTx, []any{MetaKeyNextTxID})
	if err == heap.ErrKeyNotFound {
		// Bootstrap: insert nextTxID = 1 using Tx{1}.
		// Value is 1, not 0, to prevent first catalog write from reusing txID=1.
		err = c.metaHandle.Insert(ctx, heap.Tx{ID: 1}, []any{MetaKeyNextTxID, uint64(1)})
		if err != nil {
			return fmt.Errorf("catalog: bootstrap meta: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("catalog: check meta: %w", err)
	}

	return nil
}
