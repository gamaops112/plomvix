package catalog

import (
	"context"
	"fmt"
	"math"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
)

// cache holds the in-memory catalog state with pending operation tracking.
type cache struct {
	tables        map[string]TableInfo
	users         map[string]UserInfo
	pendingTables map[string]struct{}
	pendingUsers  map[string]struct{}
	nextTableID   uint64
	nextUserID    uint64
}

func newCache() *cache {
	return &cache{
		tables:        make(map[string]TableInfo),
		users:         make(map[string]UserInfo),
		pendingTables: make(map[string]struct{}),
		pendingUsers:  make(map[string]struct{}),
		nextTableID:   1,
		nextUserID:    1,
	}
}

// copyTableInfo returns a deep copy with cloned []byte fields.
func copyTableInfo(t TableInfo) TableInfo {
	cp := t
	if t.SchemaPayload != nil {
		cp.SchemaPayload = make([]byte, len(t.SchemaPayload))
		copy(cp.SchemaPayload, t.SchemaPayload)
	}
	return cp
}

// copyUserInfo returns a deep copy with cloned []byte fields.
func copyUserInfo(u UserInfo) UserInfo {
	cp := u
	if u.PasswordHash != nil {
		cp.PasswordHash = make([]byte, len(u.PasswordHash))
		copy(cp.PasswordHash, u.PasswordHash)
	}
	return cp
}

// loadCache scans system tables and populates an in-memory cache.
// Uses Get for meta (single key) to work around known scan issue.
func (c *catalog) loadCache(ctx context.Context) (*cache, error) {
	cc := newCache()

	maxTx := heap.Tx{ID: math.MaxUint64}

	// Load meta: use Get (single-key, reliable).
	vals, err := c.metaHandle.Get(ctx, maxTx, []any{MetaKeyNextTxID})
	if err == nil {
		c.nextTxID = vals[1].(uint64)
	} else if err != heap.ErrKeyNotFound {
		return nil, fmt.Errorf("catalog: get meta: %w", err)
	}

	// Load tables.
	tableRows, err := c.tablesHandle.Scan(ctx, maxTx)
	if err != nil {
		return nil, fmt.Errorf("catalog: scan tables: %w", err)
	}
	defer tableRows.Close()
	for tableRows.Next() {
		vals := tableRows.Values()
		ti := TableInfo{
			TableID:       vals[0].(uint64),
			EngineName:    vals[1].(string),
			TableName:     vals[2].(string),
			SchemaPayload: vals[3].([]byte),
		}
		cc.tables[ti.TableName] = copyTableInfo(ti)
		if ti.TableID >= cc.nextTableID {
			cc.nextTableID = ti.TableID + 1
		}
	}

	// Load users.
	userRows, err := c.usersHandle.Scan(ctx, maxTx)
	if err != nil {
		return nil, fmt.Errorf("catalog: scan users: %w", err)
	}
	defer userRows.Close()
	for userRows.Next() {
		vals := userRows.Values()
		ui := UserInfo{
			UserID:       vals[0].(uint64),
			Username:     vals[1].(string),
			PasswordHash: vals[2].([]byte),
			IsAdmin:      vals[3].(uint64) == 1,
		}
		cc.users[ui.Username] = copyUserInfo(ui)
		if ui.UserID >= cc.nextUserID {
			cc.nextUserID = ui.UserID + 1
		}
	}

	return cc, nil
}
