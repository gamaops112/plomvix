package engine

import (
	"testing"
)

func TestSchemaDeepCopy(t *testing.T) {
	s := Schema{Columns: []Column{{Name: "id", Type: TypeInt64}, {Name: "name", Type: TypeString}}}
	cp := s.DeepCopy()
	cp.Columns[0].Name = "changed"
	if s.Columns[0].Name != "id" {
		t.Error("DeepCopy mutated original")
	}
}

func TestDatumDeepCopy(t *testing.T) {
	b := []byte{1, 2, 3}
	d := Datum{Type: TypeBytes, Value: b}
	cp := d.DeepCopy()
	b[0] = 99
	if cp.Value.([]byte)[0] != 1 {
		t.Error("DeepCopy did not copy []byte")
	}
}

func TestRowDeepCopy(t *testing.T) {
	r := Row{Datums: []Datum{{Type: TypeInt64, Value: int64(42)}, {Type: TypeBytes, Value: []byte{1, 2}}}}
	cp := r.DeepCopy()
	cp.Datums[0] = Datum{Type: TypeBool, Value: true}
	cp.Datums[1].Value.([]byte)[0] = 99
	if r.Datums[0].Value.(int64) != 42 || r.Datums[1].Value.([]byte)[0] != 1 {
		t.Error("Row DeepCopy leaked mutations")
	}
}
