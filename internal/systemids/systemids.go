// Package systemids defines neutral constants for reserved Plomvix system
// table IDs. Both the Catalog and Vacuum Manager import this package to
// prevent duplication and drift across packages.
package systemids

// Reserved system table ID ranges.
const (
	SystemTableMinID uint64 = 1
	SystemTableMaxID uint64 = 999

	// Reserved system table IDs.
	SystemTableTables  uint64 = 1 // _plomvix_tables
	SystemTableColumns uint64 = 2 // _plomvix_columns
	SystemTableUsers   uint64 = 3 // _plomvix_users
)
