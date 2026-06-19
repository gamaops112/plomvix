Here is the complete, highly detailed handoff document. You can copy and paste the entire block below directly to Gemini. It is specifically engineered to prevent it from falling into the traps of the deprecated documents and to give it the exact architectural context needed to draft **Plan 28a: DML Setup**.

***

# 🚀 Plomvix Project: Master AI Handoff Document
**To:** Incoming AI Architect / Coding Agent (Gemini)
**From:** Outgoing Technical Lead
**Subject:** Complete Project Context, Architectural Boundaries, and Current State (Up to Plan 27b)
**Immediate Mission:** Draft and execute **Plan 28a: DML Execution Setup (`INSERT` / `UPDATE` / `DELETE`)**

Welcome to the Plomvix project. You are taking over as the Technical Lead and Architect for a from-scratch, multi-engine relational database server written in Go. 

This document contains the complete history, the strict engineering rules, the current state of the codebase, and the exact architectural boundaries you must respect. **Read this entirely before generating any code or new plans.**

---

## 1. 🚨 CRITICAL: Deprecated vs. Active Artifacts
The user has provided you with three reference files: `master_plan.md`, `master_plan2.md`, and `storage_setup.md`. 

**YOU MUST TREAT THE STORAGE AND KV PORTIONS OF THESE FILES AS DEPRECATED HISTORICAL ARTIFACTS.**

*   **The Abandoned Path:** Those files reference using **bbolt** (Plan 14) and **Pebble** (Plan 15) for the KVStore, as well as an In-Memory KV store (Plans 16-17). **This path is completely dead.** We abandoned third-party KV stores because we needed absolute, ground-up control over the WAL, TOAST overflow pages, and MVCC version slots.
*   **The Active Path:** We built a custom 4KB Pager (`internal/storage/pager`), a custom B+ Tree (`internal/engine/sql/kv`), and an Append-Only MVCC Table Heap (`internal/engine/sql/heap`) entirely from scratch. 
*   **Actionable Rule:** If you see references to bbolt, Pebble, `KVStore` interfaces, or in-memory sorted slices in those old docs, **ignore them**. Do not implement them. Do not suggest them. The active roadmap stops at Plan 17 in those docs, but **our actual project is currently at Plan 27b**.

---

## 2. High-Level Architecture
Plomvix uses a **ClickHouse-style pluggable architecture**. It is not just a SQL database; it is a server that hosts multiple specialized engines.

*   **The Global Control Plane:** Lives outside the engines. Includes the Wire Protocol (future), Global Catalog, Global SQL Parser, and Global Query Router.
*   **The Pluggable Engines:** 
    1.  **SQL Engine:** Relational row storage, standard OLTP queries. (This is what we are building).
    2.  **Metrics Engine:** (Future) Time-series optimized storage.
    3.  **Logs Engine:** (Future) Inverted-index/Full-text search storage.
*   **The Shared Storage Primitive:** The `internal/storage/pager` (Custom WAL, 4KB pages, multi-page atomicity). Every engine instantiates its own Pager for its own data files, but the underlying code is shared.
*   **The Global Catalog:** Owns `information_schema`, RBAC, and metadata for *all* engines. It uses the SQL Engine's Table Heap internally (via a `SystemHeapAdapter`) to persist its own system tables, but acts as a global service.

---

## 3. The "Golden Rules" of Plomvix Engineering
When designing new plans or reviewing code, you **MUST** strictly adhere to these rules. We have spent hours hardening these concepts; do not regress them:

1.  **The Two-Tier Structure:** Every major feature gets a `_setup.md` (Basic) and an `_enterprise.md` (Hardening). Basic establishes the API and core logic. Enterprise adds concurrency hardening, caching, edge-case safety, and production telemetry.
2.  **One Calcifying Concern Per Plan:** Do not mix storage layer changes with query layer changes. Do not mix DDL (schema) with DML (data). Do not mix parser changes with router changes.
3.  **Honest Contracts:** If a tier does not support something (e.g., strict serializable conflict detection, batch inserts), explicitly document it as a "Known Trade-off" or "Deferred". Never fake a capability.
4.  **Safe-Locking & Concurrency:** 
    *   Never hold a global/catalog mutex while doing Disk I/O or calling external engine callbacks.
    *   Use the "Lock -> Re-check State -> Reserve Tx/ID -> Add Pending Marker -> Unlock -> I/O -> Lock -> Publish Cache" pattern.
5.  **Deep-Copy Cache Safety:** Any `[]byte` or slice returned from an in-memory cache (like the Catalog or Parser wrapper) MUST be a deep copy to prevent callers from mutating global state.
6.  **No Unjustified Dependencies:** We use the Go standard library for almost everything. Exceptions are explicitly approved (e.g., `vitess.io/vitess/go/vt/sqlparser` for SQL parsing, `golang.org/x/crypto/bcrypt` for auth). Core storage primitives are strictly custom.

---

## 4. Current Project State (The Active Roadmap)

### ✅ Phase 1: Core Foundations (COMPLETED)
*   **Plans 1-9:** Config (TOML), Logger (`*slog.Logger`), Lifecycle Manager (state machine, panic recovery), Runtime composition, and OS Signal handling.

### ✅ Phase 2: Custom Storage & Key Encoding (COMPLETED)
*   **Plans 10-13:** SQL Key Encoding (Wire-level formats, sort-safe composites, MVCC version slots).
*   **Plans 14-15:** Custom Pager (4KB page manager, custom WAL, multi-page atomicity). *[Replaces old bbolt]*
*   **Plans 16-19:** Custom On-Disk B+Tree & KVStore (16MB TOAST overflow, shadow-paging Compact). *[Replaces old Pebble]*
*   **Plans 20-21:** Table Heap (Relational rows mapped to KV, Append-only MVCC, NULL bitmasks, Vacuum).

### ✅ Phase 3: Control Plane & Read Path (COMPLETED)
*   **Plans 22-23:** Global Catalog (System tables, Engine Registry, RBAC, Meta-first Tx).
*   **Plans 24-25:** Global SQL Parser (Vitess wrapper, Normalization, Fingerprinting).
*   **Plans 26a-26b:** Global Router & Volcano Planner (Setup & Enterprise). Includes the Plan Cache, Schema Version Pinning, and the API shift to `*engine.Result`.

### ✅ Phase 4: The Write Path - DDL (COMPLETED)
*   **Plans 27a-27b:** DDL Execution (Setup & Enterprise). Unblocked `CREATE TABLE` and `DROP TABLE`. Implemented the `TxManager`, physical Heap initialization, Catalog metadata caching, System Table bootstrapping (`SystemHeapAdapter`), and the background `VacuumManager` for orphaned files.

### 🔴 Phase 4: The Write Path - DML (YOUR IMMEDIATE MISSION)
*   **Plan 28a:** DML Execution Setup (`INSERT` / `UPDATE` / `DELETE`).
*   **Plan 28b:** DML Execution Enterprise (Batching, Strict Serializable conflict detection, Heap compaction).

---

## 5. Key Interfaces & Contracts (Your Cheat Sheet)

To prevent you from guessing the API, here are the exact interfaces you must interact with for DML:

### The API Shift (From Plan 27a)
The Engine and Router no longer return just a `RowStream`. They return a `Result` to accommodate non-streaming DDL/DML.
```go
package engine

type TxContext struct {
    ReadTxID  uint64 // Used for SELECT snapshot isolation
    WriteTxID uint64 // Used for DDL/DML mutation timestamps
}

type Result struct {
    Stream       RowStream // Non-nil for SELECT. nil for DDL/DML.
    RowsAffected int64     // Non-zero for DML. 0 for DDL/SELECT.
    Message      string    // Human-readable status for DDL.
}

type Engine interface {
    Name() string 
    Execute(ctx context.Context, req *Request) (*Result, error)
}
```

### The Catalog & Schema Versioning
The Catalog tracks metadata. Every DDL/DML that changes schema bumps the version.
```go
package catalog

type Catalog interface {
    GetTable(ctx context.Context, name string) (*TableInfo, error)
    CheckPermission(ctx context.Context, userID uint64, tableID uint64, action Action) error
    // ... DDL methods ...
}

type SchemaVersionProvider interface {
    SchemaVersion() uint64
}
```

### The MVCC Table Heap (Crucial for DML)
The Planner uses `TableHeap` for scanning (Reads). But for DML (Writes), the SQL Engine interacts directly with the underlying MVCC Heap to append new versions or tombstones.
```go
// The Heap supports MVCC appends. 
// To INSERT: Append a new tuple with WriteTxID.
// To DELETE: Append a "tombstone" tuple (NULL value) with a newer WriteTxID for the same PK.
// To UPDATE: Append a new version of the tuple with a newer WriteTxID.
```

---

## 6. Your Immediate Mission: Plan 28a (DML Setup)

Your first task is to draft the calcified markdown document for **`agent/dml_setup.md`**. 

### Context for the DML Planner/Executor:
1.  **Router Unblocking:** The Router currently returns `ErrUnsupportedStatement` for `StmtDML`. You must unblock `INSERT`, `UPDATE`, and `DELETE`.
2.  **`INSERT` Execution:** 
    *   Parse the Vitess `INSERT` AST.
    *   Resolve the target table's schema from the Catalog.
    *   Validate that the incoming values match the column types.
    *   Allocate a `WriteTxID` from the `TxManager`.
    *   Encode the row using our SQL Key formats and append it to the Table Heap.
    *   Return `&engine.Result{RowsAffected: 1}`.
3.  **`UPDATE` and `DELETE` Execution (The MVCC Way):**
    *   Because our Table Heap is **Append-Only MVCC**, we *never* modify or delete bytes in place.
    *   **DELETE:** Use the existing Volcano `SeqScan` and `Filter` nodes (from Plan 26) to find the target rows, and then append a **Tombstone** tuple with a newer `WriteTxID`.
    *   **UPDATE:** Find the target rows, and append a **new version** of the tuple with the updated values and a newer `WriteTxID`.
4.  **Honest Contracts (What to defer to 28b):** 
    *   Basic tier operates in **Snapshot Isolation**. Strict Serializable conflict detection (checking if another transaction modified the row between read and write) is deferred.
    *   No batch inserts (e.g., `INSERT INTO t VALUES (...), (...)`). Only single-row inserts in Basic.
    *   No subqueries in `INSERT` (e.g., `INSERT INTO t SELECT ...`).

### Instructions for Drafting the Plan:
1.  Follow the exact markdown format used in previous plans (Header, Honest Contracts, Deliverables, Key API & Concepts, Tasks, Completion Criteria).
2.  Ensure you include the exact Vitess AST accessors needed (e.g., `RawInsert()`, `RawUpdate()`, `RawDelete()`).
3.  Define the exact type-mapping and validation logic for `INSERT` values against the `engine.Schema`.
4.  Explicitly define how `UPDATE` and `DELETE` will use the existing Volcano pipeline to find rows, and how they will append MVCC versions/tombstones to the Heap.
5.  Include a task for updating the Router to allow `StmtDML` and route it to the SQL engine.

---

## 7. Execution Directives for Gemini

1.  **Acknowledge the Handoff:** Start your first response by confirming you understand the Golden Rules, the abandonment of bbolt/Pebble, and the current state of Plan 27b.
2.  **Draft Plan 28a:** Generate the complete, calcified `agent/dml_setup.md` document in a single markdown block.
3.  **Wait for Approval:** Do not start writing Go code until the user reviews and approves the `dml_setup.md` document. The user is very strict about architectural boundaries and will review your plan for race conditions, import cycles, and missing validations.
4.  **Enforce the Two-Tier Rule:** Once 28a is approved and coded, your next task will be to draft **Plan 28b (DML Enterprise)**. Do not mix them.

**End of Handoff Document. Good luck, Gemini. Build it right.**