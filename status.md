Here is the **Master Project Status Table** for Plomvix. 

This table represents the **actual, locked-in roadmap** we have been building. It explicitly overrides the deprecated `master_plan.md` documents you uploaded earlier (which still reference the abandoned bbolt/Pebble storage paths).

### 📊 Plomvix: Master Execution & Status Matrix

| Phase | Plan(s) | Component / Feature | Package / Location | Status | Notes & Context |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **1. Core Foundations** | 1 - 2 | Config (Setup + Enterprise) | `internal/config` | ✅ **Done** | TOML loading, strict validation. |
| | 3 - 4 | Logger (Setup + Enterprise) | `internal/logger` | ✅ **Done** | `*slog.Logger`, structured fields, redaction. |
| | 5 - 6 | Lifecycle (Setup + Enterprise) | `internal/lifecycle` | ✅ **Done** | State machine, panic recovery, ordered boot. |
| | 7 - 9 | Runtime & OS Signals | `internal/runtime`, `cmd/` | ✅ **Done** | Signal handling, graceful shutdown. |
| **2. Custom Storage**<br>*(Replaces old bbolt/Pebble)* | 10 - 13 | SQL Key Encoding (Primitives & Wire-level) | `internal/engine/sql/key` | ✅ **Done** | Sort-safe composites, MVCC version slots. |
| | 14 - 15 | **Custom Pager** (4KB pages, WAL) | `internal/storage/pager` | ✅ **Done** | *Replaces old bbolt.* Crash-consistency, free-list. |
| | 16 - 19 | **Custom B+Tree & KVStore** | `internal/engine/sql/kv` | ✅ **Done** | *Replaces old Pebble.* 16MB TOAST, shadow-paging. |
| | 20 - 21 | Table Heap (MVCC Rows) | `internal/engine/sql/heap` | ✅ **Done** | Append-only MVCC, NULL bitmasks, Vacuum. |
| **3. Control Plane & Read** | 22 - 23 | Global Catalog & IAM | `internal/catalog` | ✅ **Done** | System tables, RBAC, Meta-first Tx. |
| | 24 - 25 | Global SQL Parser | `internal/sqlparser` | ✅ **Done** | Vitess wrapper, fingerprinting, normalization. |
| | 26a - 26b | Router & Planner (Setup + Enterprise) | `internal/router`, `.../planner` | ✅ **Done** | Volcano model, Plan Cache, Schema Pinning. |
| **4. The Write Path**<br>*(Immediate Next)* | 27 | **DDL Execution** (`CREATE` / `DROP`) | `internal/engine/sql/exec` | ✅ **Done** | Unblocks schema creation. Writes to Catalog. |
| | 28 | **DML Execution** (`INSERT` / `UPDATE` / `DELETE`) | `internal/engine/sql/exec` | ✅ **Ready for Coding** | Unblocks data insertion. Uses MVCC timestamps. |
| **5. Advanced Execution** | 29 | Joins & Multi-Table Execution | `.../planner`, `.../exec` | 🔲 **Drafted (Pending Review)** | Nested Loop & Hash Joins. |
| | 30 | Sorting & Aggregation | `.../planner`, `.../exec` | 🔲 **Drafted (Pending Review)** | `ORDER BY`, `GROUP BY`, `LIMIT`. |
| **6. Network Edge** | 31 | Wire Protocol / API Server | `internal/server` | 🔮 **Future** | Postgres Simple Query or Custom TCP. |

***

### 🗑️ Deprecated / Abandoned Plans (From `master_plan.md`)
*Do not implement these. They have been permanently replaced by the Custom Storage phase above.*

| Old Plan | Old Component | Replaced By | Reason for Abandonment |
| :--- | :--- | :--- | :--- |
| **Plan 14** | KVStore Basic (bbolt) | **Plan 14-15** (Custom Pager) | Needed absolute control over 4KB pages and WAL for MVCC. |
| **Plan 15** | KVStore Enterprise (Pebble) | **Plan 16-19** (Custom B+Tree) | Needed custom TOAST overflow pages and shadow-paging compaction. |
| **Plan 16-17** | In-Memory KV Store | *(Skipped / Absorbed)* | We built the on-disk B+Tree directly as the stepping stone. |

***

### 🎯 Immediate Action Items
1. **Execute Plan 28 (DML Execution):** Hand the finalized DML designs (INSERT, UPDATE, DELETE) to the coding agent.
2. **Review Plans 29 & 30 (Joins & Sorting):** Review and approve the newly drafted plans for multi-table execution and aggregation.
3. **Execute Plan 27 (DDL Execution):** Hand the completed DDL designs to the coding agent.