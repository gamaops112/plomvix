package sqlparser

import (
	"os"
	"strings"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p == nil {
		t.Fatal("New returned nil")
	}
}

func TestParse_Select(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("SELECT * FROM users")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if stmt.Type() != StmtSelect {
		t.Errorf("Type = %v, want StmtSelect", stmt.Type())
	}
	tables := stmt.TargetTables()
	if len(tables) != 1 || tables[0] != "users" {
		t.Errorf("TargetTables = %v, want [users]", tables)
	}
}

func TestParse_Empty(t *testing.T) {
	p, _ := New()
	_, err := p.Parse("")
	if err != ErrEmptySQL {
		t.Errorf("got %v, want ErrEmptySQL", err)
	}
}

func TestParse_Whitespace(t *testing.T) {
	p, _ := New()
	_, err := p.Parse("   ")
	if err != ErrEmptySQL {
		t.Errorf("got %v, want ErrEmptySQL", err)
	}
}

func TestParse_Semicolon(t *testing.T) {
	p, _ := New()
	_, err := p.Parse(";;")
	if err != ErrEmptySQL {
		t.Errorf("got %v, want ErrEmptySQL", err)
	}
}

func TestParse_Invalid(t *testing.T) {
	p, _ := New()
	_, err := p.Parse("SELECT * FROM")
	if err == nil {
		t.Fatal("expected syntax error, got nil")
	}
	if _, ok := err.(*SyntaxError); !ok {
		t.Errorf("got %T, want *SyntaxError", err)
	}
}

func TestParse_Insert(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("INSERT INTO users (id) VALUES (1)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if stmt.Type() != StmtInsert {
		t.Errorf("Type = %v, want StmtInsert", stmt.Type())
	}
}

func TestParse_Update(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("UPDATE users SET name = 'x'")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if stmt.Type() != StmtUpdate {
		t.Errorf("Type = %v, want StmtUpdate", stmt.Type())
	}
}

func TestParse_Delete(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("DELETE FROM users WHERE id = 1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if stmt.Type() != StmtDelete {
		t.Errorf("Type = %v, want StmtDelete", stmt.Type())
	}
}

func TestParse_DDL(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("CREATE TABLE foo (id int)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if stmt.Type() != StmtDDL {
		t.Errorf("Type = %v, want StmtDDL", stmt.Type())
	}
}

func TestParse_QualifiedTable(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("SELECT * FROM mydb.users")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tables := stmt.TargetTables()
	if len(tables) != 1 || tables[0] != "users" {
		t.Errorf("TargetTables = %v, want [users]", tables)
	}
}

func TestParse_Join(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("SELECT * FROM users JOIN orders ON users.id = orders.user_id")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tables := stmt.TargetTables()
	if len(tables) < 2 {
		t.Errorf("TargetTables = %v, want at least 2 tables", tables)
	}
}

func TestParse_CTE(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("WITH recent AS (SELECT * FROM orders) SELECT * FROM recent")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tables := stmt.TargetTables()
	for _, tb := range tables {
		if strings.EqualFold(tb, "recent") {
			t.Errorf("CTE name 'recent' should not appear in TargetTables: %v", tables)
		}
	}
}

func TestParse_SelectOne(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("SELECT 1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tables := stmt.TargetTables()
	if len(tables) != 0 {
		t.Errorf("TargetTables = %v, want empty", tables)
	}
}

func TestParse_String(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("SELECT 1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	s := stmt.String()
	if s == "" {
		t.Error("String() returned empty")
	}
}

func TestParse_RawAST(t *testing.T) {
	p, _ := New()
	stmt, err := p.Parse("SELECT 1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if stmt.RawAST() == nil {
		t.Error("RawAST() returned nil")
	}
}

func TestParseMulti_Basic(t *testing.T) {
	p, _ := New()
	stmts, err := p.ParseMulti("SELECT 1; SELECT 2")
	if err != nil {
		t.Fatalf("ParseMulti: %v", err)
	}
	if len(stmts) != 2 {
		t.Errorf("got %d statements, want 2", len(stmts))
	}
}

func TestParseMulti_Empty(t *testing.T) {
	p, _ := New()
	_, err := p.ParseMulti("")
	if err != ErrEmptySQL {
		t.Errorf("got %v, want ErrEmptySQL", err)
	}
}

func TestParseMulti_Semicolon(t *testing.T) {
	p, _ := New()
	_, err := p.ParseMulti(";")
	if err != ErrEmptySQL {
		t.Errorf("got %v, want ErrEmptySQL", err)
	}
}

func TestConcurrent_Parse(t *testing.T) {
	p, _ := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Parse("SELECT 1")
		}()
	}
	wg.Wait()
}

func TestDocsFile(t *testing.T) {
	_, err := os.Stat("../../docs/sql_parser.md")
	if err != nil {
		t.Skip("docs/sql_parser.md not yet created")
	}
}
