package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
)

const systemUserID uint64 = 0

func (c *catalog) SchemaVersion() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.schemaVersion
}

func (c *catalog) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	if c.starting {
		c.mu.Unlock()
		return ErrCatalogAlreadyStarted
	}
	c.starting = true
	c.mu.Unlock()
	if err := c.bootstrap(ctx); err != nil {
		c.mu.Lock()
		c.starting = false
		c.mu.Unlock()
		return err
	}
	cc, err := c.loadCache(ctx)
	if err != nil {
		c.mu.Lock()
		c.starting = false
		c.mu.Unlock()
		return err
	}
	c.mu.Lock()
	c.cache = cc
	c.started = true
	c.starting = false
	c.mu.Unlock()
	return nil
}

func (c *catalog) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return nil
	}
	if c.cache != nil && (len(c.cache.pendingTables) > 0 || len(c.cache.pendingUsers) > 0) {
		return ErrConflict
	}
	c.started = false
	c.starting = false
	c.cache = nil
	return nil
}

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func (c *catalog) auditLog(ctx context.Context, txID uint64, action, targetType, targetName string) error {
	ts := uint64(time.Now().Unix())
	row := []any{c.cache.nextAuditLogID, txID, systemUserID, action, targetType, targetName, ts}
	return c.auditHandle.Insert(ctx, heap.Tx{ID: txID}, row)
}

func (c *catalog) CreateTable(ctx context.Context, engineName, tableName string, schemaJSON []byte) error {
	if engineName == "" || tableName == "" {
		return ErrEmptyName
	}
	c.mu.RLock()
	if !c.started {
		c.mu.RUnlock()
		return ErrCatalogNotStarted
	}
	eng, ok := c.engines[engineName]
	c.mu.RUnlock()
	if !ok {
		return ErrEngineNotFound
	}
	if err := eng.ValidateSchema(schemaJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSchema, err)
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	if _, exists := c.cache.tables[tableName]; exists {
		c.mu.Unlock()
		return ErrDuplicateTable
	}
	if _, p := c.cache.pendingTables[tableName]; p {
		c.mu.Unlock()
		return ErrConflict
	}
	txID := c.reserveTx()
	tableID := c.cache.nextTableID
	c.cache.pendingTables[tableName] = struct{}{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.cache != nil {
			delete(c.cache.pendingTables, tableName)
		}
		c.mu.Unlock()
	}()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	version := uint64(1)
	row := []any{tableID, engineName, tableName, schemaJSON, version}
	if err := c.tablesHandle.Insert(ctx, heap.Tx{ID: txID}, row); err != nil {
		return fmt.Errorf("catalog: insert table: %w", err)
	}
	ts := uint64(time.Now().Unix())
	histRow := []any{c.cache.nextHistoryID, tableID, version, "CREATE", schemaJSON, ts}
	if err := c.historyHandle.Insert(ctx, heap.Tx{ID: txID}, histRow); err != nil {
		return fmt.Errorf("catalog: history: %w", err)
	}
	if err := c.auditLog(ctx, txID, "CREATE_TABLE", "table", tableName); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	ti := TableInfo{TableID: tableID, EngineName: engineName, TableName: tableName, SchemaPayload: schemaJSON, SchemaVersion: version}
	c.cache.tables[tableName] = copyTableInfo(ti)
	c.cache.nextTableID++
	c.cache.nextHistoryID++
	c.cache.nextAuditLogID++
	c.schemaVersion++
	c.mu.Unlock()
	return nil
}

func (c *catalog) DropTable(ctx context.Context, tableName string) error {
	if tableName == "" {
		return ErrEmptyName
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	ti, exists := c.cache.tables[tableName]
	if !exists {
		c.mu.Unlock()
		return ErrTableNotFound
	}
	if _, p := c.cache.pendingTables[tableName]; p {
		c.mu.Unlock()
		return ErrConflict
	}
	txID := c.reserveTx()
	c.cache.pendingTables[tableName] = struct{}{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.cache != nil {
			delete(c.cache.pendingTables, tableName)
		}
		c.mu.Unlock()
	}()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	if err := c.tablesHandle.Delete(ctx, heap.Tx{ID: txID}, []any{ti.TableID}); err != nil {
		return fmt.Errorf("catalog: delete table: %w", err)
	}
	ts := uint64(time.Now().Unix())
	histRow := []any{c.cache.nextHistoryID, ti.TableID, ti.SchemaVersion + 1, "DROP", ti.SchemaPayload, ts}
	if err := c.historyHandle.Insert(ctx, heap.Tx{ID: txID}, histRow); err != nil {
		return fmt.Errorf("catalog: history: %w", err)
	}
	if err := c.auditLog(ctx, txID, "DROP_TABLE", "table", tableName); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	delete(c.cache.tables, tableName)
	c.cache.nextHistoryID++
	c.cache.nextAuditLogID++
	c.schemaVersion++
	c.mu.Unlock()
	return nil
}

func (c *catalog) GetTable(ctx context.Context, tableName string) (TableInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.started || c.cache == nil {
		return TableInfo{}, ErrCatalogNotStarted
	}
	ti, ok := c.cache.tables[tableName]
	if !ok {
		return TableInfo{}, ErrTableNotFound
	}
	return copyTableInfo(ti), nil
}

func (c *catalog) CreateUser(ctx context.Context, username, password string, isAdmin bool) error {
	if username == "" {
		return ErrEmptyName
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	if _, exists := c.cache.users[username]; exists {
		c.mu.Unlock()
		return ErrDuplicateUser
	}
	if _, p := c.cache.pendingUsers[username]; p {
		c.mu.Unlock()
		return ErrConflict
	}
	txID := c.reserveTx()
	c.cache.pendingUsers[username] = struct{}{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.cache != nil {
			delete(c.cache.pendingUsers, username)
		}
		c.mu.Unlock()
	}()
	hash, err := genBcryptHash(password)
	if err != nil {
		return fmt.Errorf("catalog: bcrypt: %w", err)
	}
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	userID := c.cache.nextUserID
	row := []any{userID, username, hash, boolToUint64(isAdmin)}
	if err := c.usersHandle.Insert(ctx, heap.Tx{ID: txID}, row); err != nil {
		return fmt.Errorf("catalog: insert user: %w", err)
	}
	if err := c.auditLog(ctx, txID, "CREATE_USER", "user", username); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	ui := UserInfo{UserID: userID, Username: username, PasswordHash: hash, IsAdmin: isAdmin}
	c.cache.users[username] = copyUserInfo(ui)
	c.cache.usersByID[userID] = copyUserInfo(ui)
	c.cache.nextUserID++
	c.cache.nextAuditLogID++
	c.mu.Unlock()
	return nil
}

func (c *catalog) Authenticate(ctx context.Context, username, password string) (UserInfo, error) {
	c.mu.RLock()
	if !c.started || c.cache == nil {
		c.mu.RUnlock()
		return UserInfo{}, ErrCatalogNotStarted
	}
	ui, ok := c.cache.users[username]
	c.mu.RUnlock()
	if !ok {
		return UserInfo{}, ErrAuthFailed
	}
	if !verifyPassword(ui.PasswordHash, password) {
		return UserInfo{}, ErrAuthFailed
	}
	if isLegacyHash(ui.PasswordHash) {
		c.migrateLegacyPassword(ctx, ui, password)
	}
	return copyUserInfo(ui), nil
}

func (c *catalog) migrateLegacyPassword(ctx context.Context, ui UserInfo, password string) error {
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	if _, p := c.cache.pendingUsers[ui.Username]; p {
		c.mu.Unlock()
		return ErrConflict
	}
	txID := c.reserveTx()
	c.cache.pendingUsers[ui.Username] = struct{}{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.cache != nil {
			delete(c.cache.pendingUsers, ui.Username)
		}
		c.mu.Unlock()
	}()
	hash, err := genBcryptHash(password)
	if err != nil {
		return err
	}
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	newRow := []any{ui.UserID, ui.Username, hash, boolToUint64(ui.IsAdmin)}
	if err := c.usersHandle.Update(ctx, heap.Tx{ID: txID}, []any{ui.UserID}, newRow); err != nil {
		return fmt.Errorf("catalog: update user: %w", err)
	}
	c.mu.Lock()
	ui.PasswordHash = hash
	c.cache.users[ui.Username] = copyUserInfo(ui)
	c.cache.usersByID[ui.UserID] = copyUserInfo(ui)
	c.mu.Unlock()
	return nil
}

func (c *catalog) CreateRole(ctx context.Context, roleName string) error {
	if roleName == "" {
		return ErrEmptyName
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	if _, exists := c.cache.roles[roleName]; exists {
		c.mu.Unlock()
		return ErrDuplicateRole
	}
	txID := c.reserveTx()
	c.mu.Unlock()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	roleID := c.cache.nextRoleID
	if err := c.rolesHandle.Insert(ctx, heap.Tx{ID: txID}, []any{roleID, roleName}); err != nil {
		return fmt.Errorf("catalog: insert role: %w", err)
	}
	if err := c.auditLog(ctx, txID, "CREATE_ROLE", "role", roleName); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	ri := RoleInfo{RoleID: roleID, RoleName: roleName}
	c.cache.roles[roleName] = ri
	c.cache.rolesByID[roleID] = ri
	c.cache.nextRoleID++
	c.cache.nextAuditLogID++
	c.mu.Unlock()
	return nil
}

func (c *catalog) DropRole(ctx context.Context, roleName string) error {
	if roleName == "" {
		return ErrEmptyName
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	ri, exists := c.cache.roles[roleName]
	if !exists {
		c.mu.Unlock()
		return ErrRoleNotFound
	}
	txID := c.reserveTx()
	c.mu.Unlock()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	for _, gi := range c.cache.grants[ri.RoleID] {
		c.grantsHandle.Delete(ctx, heap.Tx{ID: txID}, []any{gi.GrantID})
	}
	for k, urID := range c.cache.userRoleAssignments {
		if k.roleID == ri.RoleID {
			c.userRolesHandle.Delete(ctx, heap.Tx{ID: txID}, []any{urID})
		}
	}
	if err := c.rolesHandle.Delete(ctx, heap.Tx{ID: txID}, []any{ri.RoleID}); err != nil {
		return fmt.Errorf("catalog: delete role: %w", err)
	}
	if err := c.auditLog(ctx, txID, "DROP_ROLE", "role", roleName); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	delete(c.cache.roles, roleName)
	delete(c.cache.rolesByID, ri.RoleID)
	delete(c.cache.grants, ri.RoleID)
	for k := range c.cache.grantsByKey {
		if k.roleID == ri.RoleID {
			delete(c.cache.grantsByKey, k)
		}
	}
	for k := range c.cache.userRoleAssignments {
		if k.roleID == ri.RoleID {
			delete(c.cache.userRoleAssignments, k)
		}
	}
	for uid, rids := range c.cache.userRoles {
		f := make([]uint64, 0)
		for _, r := range rids {
			if r != ri.RoleID {
				f = append(f, r)
			}
		}
		c.cache.userRoles[uid] = f
	}
	c.cache.nextAuditLogID++
	c.mu.Unlock()
	return nil
}

func (c *catalog) AssignRole(ctx context.Context, username, roleName string) error {
	if username == "" || roleName == "" {
		return ErrEmptyName
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	ui, uok := c.cache.users[username]
	ri, rok := c.cache.roles[roleName]
	if !uok || !rok {
		c.mu.Unlock()
		return ErrRoleNotFound
	}
	if _, exists := c.cache.userRoleAssignments[userRoleKey{ui.UserID, ri.RoleID}]; exists {
		c.mu.Unlock()
		return ErrDuplicateRoleAssignment
	}
	txID := c.reserveTx()
	c.mu.Unlock()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	urID := c.cache.nextUserRoleID
	if err := c.userRolesHandle.Insert(ctx, heap.Tx{ID: txID}, []any{urID, ui.UserID, ri.RoleID}); err != nil {
		return fmt.Errorf("catalog: assign role: %w", err)
	}
	if err := c.auditLog(ctx, txID, "ASSIGN_ROLE", "user_role", username+":"+roleName); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	c.cache.userRoles[ui.UserID] = append(c.cache.userRoles[ui.UserID], ri.RoleID)
	c.cache.userRoleAssignments[userRoleKey{ui.UserID, ri.RoleID}] = urID
	c.cache.nextUserRoleID++
	c.cache.nextAuditLogID++
	c.mu.Unlock()
	return nil
}

func (c *catalog) RevokeRole(ctx context.Context, username, roleName string) error {
	if username == "" || roleName == "" {
		return ErrEmptyName
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	ui, uok := c.cache.users[username]
	ri, rok := c.cache.roles[roleName]
	if !uok || !rok {
		c.mu.Unlock()
		return ErrRoleNotFound
	}
	urID, exists := c.cache.userRoleAssignments[userRoleKey{ui.UserID, ri.RoleID}]
	if !exists {
		c.mu.Unlock()
		return ErrRoleNotFound
	}
	txID := c.reserveTx()
	c.mu.Unlock()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	if err := c.userRolesHandle.Delete(ctx, heap.Tx{ID: txID}, []any{urID}); err != nil {
		return fmt.Errorf("catalog: revoke role: %w", err)
	}
	if err := c.auditLog(ctx, txID, "REVOKE_ROLE", "user_role", username+":"+roleName); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	delete(c.cache.userRoleAssignments, userRoleKey{ui.UserID, ri.RoleID})
	f := make([]uint64, 0)
	for _, r := range c.cache.userRoles[ui.UserID] {
		if r != ri.RoleID {
			f = append(f, r)
		}
	}
	c.cache.userRoles[ui.UserID] = f
	c.cache.nextAuditLogID++
	c.mu.Unlock()
	return nil
}

func (c *catalog) Grant(ctx context.Context, roleName, tableName string, action Action) error {
	if roleName == "" {
		return ErrEmptyName
	}
	if !isValidAction(action) {
		return ErrInvalidAction
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	ri, ok := c.cache.roles[roleName]
	if !ok {
		c.mu.Unlock()
		return ErrRoleNotFound
	}
	var tableID uint64
	if tableName != "" {
		ti, tok := c.cache.tables[tableName]
		if !tok {
			c.mu.Unlock()
			return ErrTableNotFound
		}
		tableID = ti.TableID
	}
	if _, exists := c.cache.grantsByKey[grantKey{ri.RoleID, tableID, action}]; exists {
		c.mu.Unlock()
		return ErrDuplicateGrant
	}
	txID := c.reserveTx()
	c.mu.Unlock()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	grantID := c.cache.nextGrantID
	if err := c.grantsHandle.Insert(ctx, heap.Tx{ID: txID}, []any{grantID, ri.RoleID, tableID, string(action)}); err != nil {
		return fmt.Errorf("catalog: insert grant: %w", err)
	}
	if err := c.auditLog(ctx, txID, "GRANT", "grant", fmt.Sprintf("%s:%s:%s", roleName, tableName, action)); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	gi := GrantInfo{GrantID: grantID, RoleID: ri.RoleID, TableID: tableID, Action: action}
	c.cache.grants[ri.RoleID] = append(c.cache.grants[ri.RoleID], gi)
	c.cache.grantsByKey[grantKey{ri.RoleID, tableID, action}] = gi
	c.cache.nextGrantID++
	c.cache.nextAuditLogID++
	c.mu.Unlock()
	return nil
}

func (c *catalog) Revoke(ctx context.Context, roleName, tableName string, action Action) error {
	if roleName == "" {
		return ErrEmptyName
	}
	if !isValidAction(action) {
		return ErrInvalidAction
	}
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	ri, ok := c.cache.roles[roleName]
	if !ok {
		c.mu.Unlock()
		return ErrRoleNotFound
	}
	var tableID uint64
	if tableName != "" {
		ti, tok := c.cache.tables[tableName]
		if !tok {
			c.mu.Unlock()
			return ErrTableNotFound
		}
		tableID = ti.TableID
	}
	gi, exists := c.cache.grantsByKey[grantKey{ri.RoleID, tableID, action}]
	if !exists {
		c.mu.Unlock()
		return ErrGrantNotFound
	}
	txID := c.reserveTx()
	c.mu.Unlock()
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: meta: %w", err)
	}
	if err := c.grantsHandle.Delete(ctx, heap.Tx{ID: txID}, []any{gi.GrantID}); err != nil {
		return fmt.Errorf("catalog: delete grant: %w", err)
	}
	if err := c.auditLog(ctx, txID, "REVOKE", "grant", fmt.Sprintf("%s:%s:%s", roleName, tableName, action)); err != nil {
		return fmt.Errorf("catalog: audit: %w", err)
	}
	c.mu.Lock()
	delete(c.cache.grantsByKey, grantKey{ri.RoleID, tableID, action})
	ff := make([]GrantInfo, 0)
	for _, g := range c.cache.grants[ri.RoleID] {
		if g.GrantID != gi.GrantID {
			ff = append(ff, g)
		}
	}
	c.cache.grants[ri.RoleID] = ff
	c.cache.nextAuditLogID++
	c.mu.Unlock()
	return nil
}

func (c *catalog) CheckPermission(ctx context.Context, userID, tableID uint64, action Action) (bool, error) {
	if !isValidAction(action) {
		return false, ErrInvalidAction
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.started || c.cache == nil {
		return false, ErrCatalogNotStarted
	}
	u, ok := c.cache.usersByID[userID]
	if !ok {
		return false, nil
	}
	if u.IsAdmin {
		return true, nil
	}
	for _, roleID := range c.cache.userRoles[userID] {
		if _, exists := c.cache.rolesByID[roleID]; !exists {
			continue
		}
		for _, gi := range c.cache.grants[roleID] {
			if (gi.TableID == 0 || gi.TableID == tableID) && gi.Action == action {
				return true, nil
			}
		}
	}
	return false, nil
}

func (c *catalog) GetSchemaHistory(ctx context.Context, tableName string) ([]SchemaHistoryEntry, error) {
	c.mu.RLock()
	ti, ok := c.cache.tables[tableName]
	c.mu.RUnlock()
	if !ok {
		return nil, ErrTableNotFound
	}
	rows, err := c.historyHandle.Scan(ctx, heap.Tx{ID: ^uint64(0)})
	if err != nil {
		return nil, fmt.Errorf("catalog: scan history: %w", err)
	}
	defer rows.Close()
	var result []SchemaHistoryEntry
	for rows.Next() {
		vals := rows.Values()
		if vals[1].(uint64) == ti.TableID {
			result = append(result, copySchemaHistoryEntry(SchemaHistoryEntry{
				HistoryID: vals[0].(uint64), TableID: vals[1].(uint64),
				Version: vals[2].(uint64), Action: vals[3].(string),
				SchemaPayload: vals[4].([]byte), Timestamp: vals[5].(uint64),
			}))
		}
	}
	return result, nil
}
