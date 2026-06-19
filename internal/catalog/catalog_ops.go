package catalog

import (
	"context"
	"fmt"

	"github.com/plomvix/plomvix/internal/engine/sql/heap"
)

// --- Start ---

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

	// Bootstrap system tables (no lock held).
	if err := c.bootstrap(ctx); err != nil {
		c.mu.Lock()
		c.starting = false
		c.mu.Unlock()
		return err
	}

	// Load cache (no lock held).
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

// --- Stop ---

func (c *catalog) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return nil
	}
	if c.cache != nil {
		if len(c.cache.pendingTables) > 0 || len(c.cache.pendingUsers) > 0 {
			return ErrConflict
		}
	}
	c.started = false
	c.starting = false
	c.cache = nil
	return nil
}

// --- CreateTable ---

func (c *catalog) CreateTable(ctx context.Context, engineName, tableName string, schemaJSON []byte) error {
	if engineName == "" || tableName == "" {
		return ErrEmptyName
	}

	// Look up engine (read lock).
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

	// Validate schema (no lock).
	if err := eng.ValidateSchema(schemaJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSchema, err)
	}

	// Reserve operation (write lock).
	c.mu.Lock()
	if !c.started || c.cache == nil {
		c.mu.Unlock()
		return ErrCatalogNotStarted
	}
	if _, exists := c.cache.tables[tableName]; exists {
		c.mu.Unlock()
		return ErrDuplicateTable
	}
	if _, pending := c.cache.pendingTables[tableName]; pending {
		c.mu.Unlock()
		return ErrConflict
	}

	txID := c.nextTxID + 1
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

	// Meta-First: persist txID before data.
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: persist txID: %w", err)
	}

	// Tx consumption: update in-memory after meta write succeeds.
	c.mu.Lock()
	c.nextTxID = txID
	c.mu.Unlock()

	rowValues := []any{tableID, engineName, tableName, schemaJSON}
	if err := c.tablesHandle.Insert(ctx, heap.Tx{ID: txID}, rowValues); err != nil {
		return fmt.Errorf("catalog: insert table row: %w", err)
	}

	// Commit to cache.
	c.mu.Lock()
	ti := TableInfo{
		TableID:       tableID,
		EngineName:    engineName,
		TableName:     tableName,
		SchemaPayload: schemaJSON,
	}
	c.cache.tables[tableName] = copyTableInfo(ti)
	c.cache.nextTableID++
	c.mu.Unlock()

	return nil
}

// --- DropTable ---

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
	if _, pending := c.cache.pendingTables[tableName]; pending {
		c.mu.Unlock()
		return ErrConflict
	}

	txID := c.nextTxID + 1
	c.cache.pendingTables[tableName] = struct{}{}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.cache != nil {
			delete(c.cache.pendingTables, tableName)
		}
		c.mu.Unlock()
	}()

	// Meta-First.
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: persist txID: %w", err)
	}

	c.mu.Lock()
	c.nextTxID = txID
	c.mu.Unlock()

	if err := c.tablesHandle.Delete(ctx, heap.Tx{ID: txID}, []any{ti.TableID}); err != nil {
		return fmt.Errorf("catalog: delete table row: %w", err)
	}

	c.mu.Lock()
	delete(c.cache.tables, tableName)
	c.mu.Unlock()

	return nil
}

// --- GetTable ---

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

// --- CreateUser ---

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
	if _, pending := c.cache.pendingUsers[username]; pending {
		c.mu.Unlock()
		return ErrConflict
	}

	txID := c.nextTxID + 1
	c.cache.pendingUsers[username] = struct{}{}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.cache != nil {
			delete(c.cache.pendingUsers, username)
		}
		c.mu.Unlock()
	}()

	// Generate password hash.
	hash, err := genPasswordHash(password)
	if err != nil {
		return err
	}

	isAdminVal := uint64(0)
	if isAdmin {
		isAdminVal = 1
	}

	// Meta-First.
	if err := c.metaHandle.Update(ctx, heap.Tx{ID: txID}, []any{MetaKeyNextTxID}, []any{MetaKeyNextTxID, txID}); err != nil {
		return fmt.Errorf("catalog: persist txID: %w", err)
	}

	c.mu.Lock()
	c.nextTxID = txID
	c.mu.Unlock()

	userID := c.cache.nextUserID
	rowValues := []any{userID, username, hash, isAdminVal}
	if err := c.usersHandle.Insert(ctx, heap.Tx{ID: txID}, rowValues); err != nil {
		return fmt.Errorf("catalog: insert user row: %w", err)
	}

	// Commit to cache.
	c.mu.Lock()
	ui := UserInfo{
		UserID:       userID,
		Username:     username,
		PasswordHash: hash,
		IsAdmin:      isAdmin,
	}
	c.cache.users[username] = copyUserInfo(ui)
	c.cache.nextUserID++
	c.mu.Unlock()

	return nil
}

// --- Authenticate ---

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

	return copyUserInfo(ui), nil
}
