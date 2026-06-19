package planner

import (
	"context"
	"io"
	"sort"

	"github.com/plomvix/plomvix/internal/engine"
)

// SortKey describes a single ORDER BY column.
type SortKey struct {
	ColIdx int
	Desc   bool
}

// SortNode buffers all rows from its child and yields them in sorted order.
type SortNode struct {
	child     Operator
	keys      []SortKey
	outSchema engine.Schema

	rows   []engine.Row
	rowIdx int
	opened bool
}

// NewSortNode creates a SortNode.
func NewSortNode(child Operator, keys []SortKey) *SortNode {
	return &SortNode{
		child:     child,
		keys:      keys,
		outSchema: child.Schema(),
	}
}

func (n *SortNode) Open(ctx context.Context) error {
	if err := n.child.Open(ctx); err != nil {
		return err
	}
	var rows []engine.Row
	for {
		r, err := n.child.Next(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		r1, r2 := rows[i], rows[j]
		for _, key := range n.keys {
			d1, d2 := r1.Datums[key.ColIdx], r2.Datums[key.ColIdx]
			if datumEqual(d1, d2) {
				continue
			}
			// NULLs sort first for ASC, last for DESC.
			n1, n2 := d1.Value == nil, d2.Value == nil
			if n1 != n2 {
				if key.Desc {
					return n2 // NULL last for DESC
				}
				return n1 // NULL first for ASC
			}
			less := datumLess(d1, d2)
			if key.Desc {
				return !less
			}
			return less
		}
		return false
	})
	n.rows = rows
	n.rowIdx = 0
	n.opened = true
	return nil
}

func (n *SortNode) Next(ctx context.Context) (engine.Row, error) {
	if !n.opened || n.rowIdx >= len(n.rows) {
		return engine.Row{}, io.EOF
	}
	r := n.rows[n.rowIdx]
	n.rowIdx++
	return r, nil
}

func (n *SortNode) Close() error {
	n.rows = nil
	n.opened = false
	return n.child.Close()
}

func (n *SortNode) Schema() engine.Schema { return n.outSchema.DeepCopy() }
