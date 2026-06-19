# Plomvix Global System Catalog

The catalog package provides the server-level control plane for Plomvix.
It manages system metadata, registers pluggable engines, and provides
schema resolution with basic authentication.

## Architecture

- System tables (_plomvix_tables, _plomvix_users, _plomvix_meta) persist
  all metadata via the Enterprise Table Heap.
- In-memory cache with deep copy guarantees for O(1) read access.
- Pluggable Engine interface for schema validation.

## Meta-First TxID Persistence

Every catalog DDL operation (CreateTable, DropTable, CreateUser) follows
a strict Meta-First protocol:

1. Reserve the next TxID and add the operation to a pending map (under lock).
2. Update the _plomvix_meta row to persist the new TxID.
3. Immediately update c.nextTxID in-memory (Tx consumption).
4. Write the actual data row (table or user).
5. Commit to the in-memory cache (under lock).

If step 4 fails, the TxID is safely "gapped" — committed on disk and in-memory
but unreferenced by any data row. This prevents TxID reuse in the same process.

## Safe Locking and Stop Draining

- The catalog NEVER holds its internal mutex while performing Heap I/O.
- DDL operations use a multi-phase lock/unlock pattern with pending maps.
- Write-lock reservation phases re-check `!started || cache == nil` to prevent
  nil-pointer panics if Stop() runs between phases.
- Stop() checks pending maps and returns ErrConflict if operations are
  in-flight, preventing cache teardown during I/O.

## Deep Copies

All cache reads return deep copies of []byte fields. Mutations to returned
values do not affect the in-memory cache.

## Authentication

Passwords are hashed using crypto/sha256 with a random 16-byte salt.
Comparison uses crypto/subtle.ConstantTimeCompare. Empty passwords are allowed.
