package catalog

import (
	"context"
	"fmt"
	"math"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
)

// bootstrap initializes the system tables and meta on a fresh Heap.
// When using NewWithStores, the system tables are already open; this
// is a no-op.
func (c *catalog) bootstrap(ctx context.Context) error {
	// If using SystemTable adapters (NewWithStores), the factory already
	// opened the physical files. Just return.
	if c.h == nil {
		return nil
	}

	var err error

	c.tablesHandle, err = c.h.OpenTable(schemaTables)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_tables: %w", err)
	}
	c.usersHandle, err = c.h.OpenTable(schemaUsers)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_users: %w", err)
	}
	c.metaHandle, err = c.h.OpenTable(schemaMeta)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_meta: %w", err)
	}
	c.rolesHandle, err = c.h.OpenTable(schemaRoles)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_roles: %w", err)
	}
	c.grantsHandle, err = c.h.OpenTable(schemaGrants)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_grants: %w", err)
	}
	c.userRolesHandle, err = c.h.OpenTable(schemaUserRoles)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_user_roles: %w", err)
	}
	c.historyHandle, err = c.h.OpenTable(schemaSchemaHistory)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_schema_history: %w", err)
	}
	c.auditHandle, err = c.h.OpenTable(schemaAuditLog)
	if err != nil {
		return fmt.Errorf("catalog: open _plomvix_audit_log: %w", err)
	}

	maxTx := heap.Tx{ID: math.MaxUint64}
	_, err = c.metaHandle.Get(ctx, maxTx, []any{MetaKeyNextTxID})
	if err == heap.ErrKeyNotFound {
		err = c.metaHandle.Insert(ctx, heap.Tx{ID: 1}, []any{MetaKeyNextTxID, uint64(1)})
		if err != nil {
			return fmt.Errorf("catalog: bootstrap meta: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("catalog: check meta: %w", err)
	}

	return nil
}
