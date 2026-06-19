package catalog

import (
	"context"
	"fmt"
	"math"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
)

// grantKey uniquely identifies a grant by role, table, and action.
type grantKey struct {
	roleID  uint64
	tableID uint64
	action  Action
}

// userRoleKey uniquely identifies a user-role assignment.
type userRoleKey struct {
	userID uint64
	roleID uint64
}

// cache holds the in-memory catalog state with pending operation tracking.
type cache struct {
	tables        map[string]TableInfo
	users         map[string]UserInfo
	usersByID     map[uint64]UserInfo
	pendingTables map[string]struct{}
	pendingUsers  map[string]struct{}
	nextTableID   uint64
	nextUserID    uint64

	// Enterprise RBAC maps.
	roles               map[string]RoleInfo
	rolesByID           map[uint64]RoleInfo
	grants              map[uint64][]GrantInfo       // roleID -> grants
	grantsByKey         map[grantKey]GrantInfo        // key -> grant (for lookup)
	userRoles           map[uint64][]uint64            // userID -> roleIDs
	userRoleAssignments map[userRoleKey]uint64         // {userID,roleID} -> user_role_id
	nextRoleID          uint64
	nextGrantID         uint64
	nextUserRoleID      uint64

	// Enterprise schema history & audit.
	nextHistoryID  uint64
	nextAuditLogID uint64
}

func newCache() *cache {
	return &cache{
		tables:              make(map[string]TableInfo),
		users:               make(map[string]UserInfo),
		usersByID:           make(map[uint64]UserInfo),
		pendingTables:       make(map[string]struct{}),
		pendingUsers:        make(map[string]struct{}),
		roles:               make(map[string]RoleInfo),
		rolesByID:           make(map[uint64]RoleInfo),
		grants:              make(map[uint64][]GrantInfo),
		grantsByKey:         make(map[grantKey]GrantInfo),
		userRoles:           make(map[uint64][]uint64),
		userRoleAssignments: make(map[userRoleKey]uint64),
		nextTableID:         1,
		nextUserID:          1,
		nextRoleID:          1,
		nextGrantID:         1,
		nextUserRoleID:      1,
		nextHistoryID:       1,
		nextAuditLogID:      1,
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

// copyRoleInfo returns a deep copy of RoleInfo.
func copyRoleInfo(r RoleInfo) RoleInfo { return r }

// copyGrantInfo returns a deep copy of GrantInfo.
func copyGrantInfo(g GrantInfo) GrantInfo { return g }

// copySchemaHistoryEntry returns a deep copy.
func copySchemaHistoryEntry(e SchemaHistoryEntry) SchemaHistoryEntry {
	cp := e
	if e.SchemaPayload != nil {
		cp.SchemaPayload = make([]byte, len(e.SchemaPayload))
		copy(cp.SchemaPayload, e.SchemaPayload)
	}
	return cp
}

// loadCache scans system tables and populates an in-memory cache.
func (c *catalog) loadCache(ctx context.Context) (*cache, error) {
	cc := newCache()

	maxTx := heap.Tx{ID: math.MaxUint64}

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
			SchemaVersion: vals[4].(uint64),
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
		cc.usersByID[ui.UserID] = copyUserInfo(ui)
		if ui.UserID >= cc.nextUserID {
			cc.nextUserID = ui.UserID + 1
		}
	}

	// Load roles.
	roleRows, err := c.rolesHandle.Scan(ctx, maxTx)
	if err != nil {
		return nil, fmt.Errorf("catalog: scan roles: %w", err)
	}
	defer roleRows.Close()
	for roleRows.Next() {
		vals := roleRows.Values()
		ri := RoleInfo{RoleID: vals[0].(uint64), RoleName: vals[1].(string)}
		cc.roles[ri.RoleName] = ri
		cc.rolesByID[ri.RoleID] = ri
		if ri.RoleID >= cc.nextRoleID {
			cc.nextRoleID = ri.RoleID + 1
		}
	}

	// Load grants.
	grantRows, err := c.grantsHandle.Scan(ctx, maxTx)
	if err != nil {
		return nil, fmt.Errorf("catalog: scan grants: %w", err)
	}
	defer grantRows.Close()
	for grantRows.Next() {
		vals := grantRows.Values()
		gi := GrantInfo{
			GrantID: vals[0].(uint64),
			RoleID:  vals[1].(uint64),
			TableID: vals[2].(uint64),
			Action:  Action(vals[3].(string)),
		}
		cc.grants[gi.RoleID] = append(cc.grants[gi.RoleID], gi)
		cc.grantsByKey[grantKey{gi.RoleID, gi.TableID, gi.Action}] = gi
		if gi.GrantID >= cc.nextGrantID {
			cc.nextGrantID = gi.GrantID + 1
		}
	}

	// Load user-role assignments.
	userRoleRows, err := c.userRolesHandle.Scan(ctx, maxTx)
	if err != nil {
		return nil, fmt.Errorf("catalog: scan user_roles: %w", err)
	}
	defer userRoleRows.Close()
	for userRoleRows.Next() {
		vals := userRoleRows.Values()
		urID := vals[0].(uint64)
		userID := vals[1].(uint64)
		roleID := vals[2].(uint64)
		cc.userRoles[userID] = append(cc.userRoles[userID], roleID)
		cc.userRoleAssignments[userRoleKey{userID, roleID}] = urID
		if urID >= cc.nextUserRoleID {
			cc.nextUserRoleID = urID + 1
		}
	}

	// Load schema history (just track next ID).
	histRows, err := c.historyHandle.Scan(ctx, maxTx)
	if err != nil {
		return nil, fmt.Errorf("catalog: scan schema_history: %w", err)
	}
	defer histRows.Close()
	for histRows.Next() {
		vals := histRows.Values()
		hid := vals[0].(uint64)
		if hid >= cc.nextHistoryID {
			cc.nextHistoryID = hid + 1
		}
	}

	// Load audit log (just track next ID).
	auditRows, err := c.auditHandle.Scan(ctx, maxTx)
	if err != nil {
		return nil, fmt.Errorf("catalog: scan audit_log: %w", err)
	}
	defer auditRows.Close()
	for auditRows.Next() {
		vals := auditRows.Values()
		aid := vals[0].(uint64)
		if aid >= cc.nextAuditLogID {
			cc.nextAuditLogID = aid + 1
		}
	}

	return cc, nil
}

