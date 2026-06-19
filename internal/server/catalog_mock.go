// Package server provides a PostgreSQL Wire Protocol v3.0 compatible server.
// catalog_mock.go intercepts common PG system catalog queries and returns
// hardcoded mock results to satisfy client driver startup probes.
package server

import (
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
)

// isCatalogQuery checks if a SQL query targets PG system catalog metadata.
func isCatalogQuery(sql string) bool {
	lower := strings.ToLower(sql)
	return strings.Contains(lower, "pg_catalog") ||
		strings.Contains(lower, "version()") ||
		strings.Contains(lower, "current_schema()") ||
		strings.Contains(lower, "show ") ||
		strings.Contains(lower, "pg_type") ||
		strings.Contains(lower, "pg_settings")
}

// executeMockCatalog returns static mock results for common PG system queries.
func executeMockCatalog(sql string) (engine.Schema, []engine.Row, string, bool) {
	lower := strings.ToLower(sql)

	if strings.Contains(lower, "version()") {
		schema := engine.Schema{Columns: []engine.Column{
			{Name: "version", Type: engine.TypeString},
		}}
		rows := []engine.Row{
			{Datums: []engine.Datum{
				{Type: engine.TypeString, Value: "Plomvix 0.1.0 (PostgreSQL 15.0.0 compatibility)"},
			}},
		}
		return schema, rows, "SELECT 1", true
	}

	if strings.Contains(lower, "current_schema()") || strings.Contains(lower, "current_schema") {
		schema := engine.Schema{Columns: []engine.Column{
			{Name: "current_schema", Type: engine.TypeString},
		}}
		rows := []engine.Row{
			{Datums: []engine.Datum{
				{Type: engine.TypeString, Value: "public"},
			}},
		}
		return schema, rows, "SELECT 1", true
	}

	if strings.Contains(lower, "pg_type") {
		schema := engine.Schema{Columns: []engine.Column{
			{Name: "oid", Type: engine.TypeInt64},
			{Name: "typname", Type: engine.TypeString},
		}}
		rows := []engine.Row{
			{Datums: []engine.Datum{
				{Type: engine.TypeInt64, Value: int64(16)},
				{Type: engine.TypeString, Value: "bool"},
			}},
			{Datums: []engine.Datum{
				{Type: engine.TypeInt64, Value: int64(20)},
				{Type: engine.TypeString, Value: "int8"},
			}},
			{Datums: []engine.Datum{
				{Type: engine.TypeInt64, Value: int64(701)},
				{Type: engine.TypeString, Value: "float8"},
			}},
			{Datums: []engine.Datum{
				{Type: engine.TypeInt64, Value: int64(1043)},
				{Type: engine.TypeString, Value: "varchar"},
			}},
		}
		return schema, rows, "SELECT 4", true
	}

	if strings.Contains(lower, "show ") {
		schema := engine.Schema{Columns: []engine.Column{
			{Name: "setting", Type: engine.TypeString},
		}}
		var rows []engine.Row
		if strings.Contains(lower, "client_encoding") {
			rows = []engine.Row{{Datums: []engine.Datum{{Type: engine.TypeString, Value: "UTF8"}}}}
		} else if strings.Contains(lower, "standard_conforming_strings") {
			rows = []engine.Row{{Datums: []engine.Datum{{Type: engine.TypeString, Value: "on"}}}}
		} else if strings.Contains(lower, "datestyle") {
			rows = []engine.Row{{Datums: []engine.Datum{{Type: engine.TypeString, Value: "ISO, YMD"}}}}
		} else {
			rows = []engine.Row{{Datums: []engine.Datum{{Type: engine.TypeString, Value: ""}}}}
		}
		return schema, rows, "SELECT 1", true
	}

	return engine.Schema{}, nil, "", false
}
