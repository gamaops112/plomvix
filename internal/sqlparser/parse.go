package sqlparser

import (
	"strings"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

func isEmptySQL(sql string) bool {
	s := strings.TrimSpace(sql)
	if s == "" {
		return true
	}
	for _, r := range s {
		if r != ';' && r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

func (v *vitessParser) Parse(sql string) (Statement, error) {
	if isEmptySQL(sql) {
		return nil, ErrEmptySQL
	}
	stmt, err := v.p.Parse(sql)
	if err != nil {
		return nil, &SyntaxError{Message: err.Error(), Line: 1, Column: 1}
	}
	return &stmtWrapper{ast: stmt, sql: sql}, nil
}

func (v *vitessParser) ParseMulti(sql string) ([]Statement, error) {
	if isEmptySQL(sql) {
		return nil, ErrEmptySQL
	}
	pieces, err := v.p.SplitStatementToPieces(sql)
	if err != nil {
		return nil, &SyntaxError{Message: err.Error(), Line: 1, Column: 1}
	}
	var results []Statement
	for _, piece := range pieces {
		trimmed := strings.TrimSpace(piece)
		if trimmed == "" || trimmed == ";" {
			continue
		}
		stmt, parseErr := v.Parse(trimmed)
		if parseErr != nil {
			return nil, parseErr
		}
		results = append(results, stmt)
	}
	if len(results) == 0 {
		return nil, ErrEmptySQL
	}
	return results, nil
}

func (s *stmtWrapper) Type() StmtType {
	switch s.ast.(type) {
	case *vitess.Select:
		return StmtSelect
	case *vitess.Insert:
		return StmtInsert
	case *vitess.Update:
		return StmtUpdate
	case *vitess.Delete:
		return StmtDelete
	default:
		if isDDL(s.ast) {
			return StmtDDL
		}
		return StmtUnknown
	}
}

func isDDL(stmt vitess.Statement) bool {
	switch stmt.(type) {
	case *vitess.CreateTable, *vitess.CreateView, *vitess.AlterTable,
		*vitess.DropTable, *vitess.DropView:
		return true
	}
	return false
}

func (s *stmtWrapper) RawAST() any { return s.ast }

func (s *stmtWrapper) String() string {
	return vitess.String(s.ast)
}

func (s *stmtWrapper) TargetTables() []string {
	rawTables := vitess.ExtractAllTables(s.ast)
	cteNames := make(map[string]bool)
	collectCTENames(s.ast, cteNames)
	seen := make(map[string]bool)
	var result []string
	for _, t := range rawTables {
		name := strings.ToLower(t)
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}
		// Exclude CTE names and the MySQL "dual" pseudo-table.
		if cteNames[name] || name == "dual" {
			continue
		}
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

func collectCTENames(node vitess.SQLNode, names map[string]bool) {
	if node == nil {
		return
	}
	vitess.Walk(func(node vitess.SQLNode) (bool, error) {
		if cte, ok := node.(*vitess.CommonTableExpr); ok {
			names[strings.ToLower(cte.ID.String())] = true
		}
		return true, nil
	}, node)
}
