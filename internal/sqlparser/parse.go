package sqlparser

import (
	"strings"
	"unicode/utf8"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

func isEmptySQL(sql string) bool {
	s := strings.TrimSpace(sql)
	if s == "" { return true }
	for _, r := range s {
		if r != ';' && r != ' ' && r != '\t' && r != '\n' && r != '\r' { return false }
	}
	return true
}

// mapVitessError creates a ParseError from a Vitess error, preserving the byte offset.
func mapVitessError(err error, sql string, offset int) *ParseError {
	pe := &ParseError{Message: err.Error(), Kind: "syntax", Offset: offset, Line: 1, Column: 1, Cause: err}
	if offset >= 0 && offset <= len(sql) {
		pe.Line = strings.Count(sql[:offset], "\n") + 1
		lastNL := strings.LastIndex(sql[:offset], "\n")
		colBytes := sql[lastNL+1 : offset]
		pe.Column = utf8.RuneCountInString(colBytes) + 1
	}
	return pe
}

// Parse parses a single SQL statement with strict DDL enforcement.
func (v *vitessParser) Parse(sql string) (Statement, error) {
	if isEmptySQL(sql) { return nil, ErrEmptySQL }

	stmt, err := v.p.Parse(sql)
	if err != nil {
		offset := -1
		if pe, ok := err.(interface{ Position() (int, int) }); ok {
			_, offset = pe.Position()
			// offset from Vitess is 0-based; keep as-is.
		}
		return nil, mapVitessError(err, sql, offset)
	}

	// Strict DDL: check IsFullyParsed on both DDLStatement and DBDDLStatement interfaces.
	if ddls, ok := stmt.(vitess.DDLStatement); ok && !ddls.IsFullyParsed() {
		return nil, &ParseError{Kind: "syntax", Message: "incomplete DDL", Offset: -1, Line: 1, Column: 1}
	}
	if dbddl, ok := stmt.(vitess.DBDDLStatement); ok && !dbddl.IsFullyParsed() {
		return nil, &ParseError{Kind: "syntax", Message: "incomplete DB DDL", Offset: -1, Line: 1, Column: 1}
	}

	// Semantic validation.
	if pe := validateSemantics(stmt); pe != nil {
		return nil, pe
	}

	return &stmtWrapper{ast: stmt, sql: sql, p: v.p}, nil
}

// ParseMulti parses multiple semicolon-separated statements (fail-fast).
func (v *vitessParser) ParseMulti(sql string) ([]Statement, error) {
	if isEmptySQL(sql) { return nil, ErrEmptySQL }

	pieces, err := v.p.SplitStatementToPieces(sql)
	if err != nil { return nil, mapVitessError(err, sql, -1) }

	var results []Statement
	anyParsed := false
	for _, piece := range pieces {
		trimmed := strings.TrimSpace(piece)
		if trimmed == "" || trimmed == ";" { continue }
		anyParsed = true
		stmt, parseErr := v.Parse(trimmed)
		if parseErr != nil { return nil, parseErr }
		results = append(results, stmt)
	}
	if !anyParsed { return nil, ErrEmptySQL }
	return results, nil
}

// ParseScript parses multiple statements with multi-error recovery.
func (v *vitessParser) ParseScript(sql string) ([]Statement, []*ParseError) {
	if isEmptySQL(sql) {
		return nil, []*ParseError{{Kind: "syntax", Message: ErrEmptySQL.Error(), Offset: -1, Line: 1, Column: 1}}
	}

	pieces, err := v.p.SplitStatementToPieces(sql)
	if err != nil {
		return nil, []*ParseError{mapVitessError(err, sql, -1)}
	}

	var stmts []Statement
	var errs []*ParseError
	prevEnd := 0

	for _, piece := range pieces {
		trimmed := strings.TrimSpace(piece)
		if trimmed == "" || trimmed == ";" { continue }

		pieceStart := strings.Index(sql[prevEnd:], trimmed)
		if pieceStart < 0 { pieceStart = prevEnd } else { pieceStart += prevEnd }

		stmt, parseErr := v.Parse(trimmed)
		if parseErr != nil {
			if pe, ok := parseErr.(*ParseError); ok {
				absOffset := pieceStart
				if pe.Kind == "syntax" && pe.Offset >= 0 {
					absOffset = pieceStart + pe.Offset
				}
				pe.Offset = absOffset
				if absOffset >= 0 && absOffset <= len(sql) {
					pe.Line = strings.Count(sql[:absOffset], "\n") + 1
					lastNL := strings.LastIndex(sql[:absOffset], "\n")
					colBytes := sql[lastNL+1 : absOffset]
					pe.Column = utf8.RuneCountInString(colBytes) + 1
				}
				errs = append(errs, pe)
			} else {
				errs = append(errs, &ParseError{Kind: "syntax", Message: parseErr.Error(), Offset: pieceStart, Line: 1, Column: 1})
			}
		} else {
			stmts = append(stmts, stmt)
		}
		prevEnd = pieceStart + len(trimmed)
	}

	if len(stmts) == 0 && len(errs) == 0 { return nil, []*ParseError{{Kind: "syntax", Message: ErrEmptySQL.Error(), Offset: -1, Line: 1, Column: 1}} }
	return stmts, errs
}

// Type returns the statement type.
func (s *stmtWrapper) Type() StmtType {
	switch s.ast.(type) {
	case *vitess.Select: return StmtSelect
	case *vitess.Insert: return StmtInsert
	case *vitess.Update: return StmtUpdate
	case *vitess.Delete: return StmtDelete
	default:
		if isDDL(s.ast) { return StmtDDL }
		return StmtUnknown
	}
}

func isDDL(stmt vitess.Statement) bool {
	switch stmt.(type) {
	case *vitess.CreateTable, *vitess.CreateView, *vitess.AlterTable, *vitess.DropTable, *vitess.DropView:
		return true
	}
	return false
}

func (s *stmtWrapper) RawAST() any { return s.ast }

func (s *stmtWrapper) String() string { return vitess.String(s.ast) }

// TargetTables extracts base table names.
func (s *stmtWrapper) TargetTables() []string {
	rawTables := vitess.ExtractAllTables(s.ast)
	cteNames := make(map[string]bool)
	collectCTENames(s.ast, cteNames)
	seen := make(map[string]bool)
	var result []string
	for _, t := range rawTables {
		name := strings.ToLower(t)
		if idx := strings.LastIndex(name, "."); idx >= 0 { name = name[idx+1:] }
		if cteNames[name] || name == "dual" { continue }
		if !seen[name] { seen[name] = true; result = append(result, name) }
	}
	return result
}

func collectCTENames(node vitess.SQLNode, names map[string]bool) {
	if node == nil { return }
	vitess.Walk(func(node vitess.SQLNode) (bool, error) {
		if cte, ok := node.(*vitess.CommonTableExpr); ok { names[strings.ToLower(cte.ID.String())] = true }
		return true, nil
	}, node)
}

