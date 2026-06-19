package planner
package planner

import (
	"context"
	"io"

	"github.com/plomvix/plomvix/internal/engine"
)

// LimitNode applies LIMIT and OFFSET to a child operator's output.
type LimitNode struct {
	child     Operator
	limit     int64
	offset    int64
	outSchema engine.Schema

	skipped int64
	emitted int64
}

// NewLimitNode creates a LimitNode.
func NewLimitNode(child Operator, limit, offset int64) *LimitNode {
	return &LimitNode{
		child:     child,
		limit:     limit,
		offset:    offset,
		outSchema: child.Schema(),
	}
}

func (n *LimitNode) Open(ctx context.Context) error {
	n.skipped = 0
	n.emitted = 0
	return n.child.Open(ctx)
}

func (n *LimitNode) Next(ctx context.Context) (engine.Row, error) {
	for {
		row, err := n.child.Next(ctx)
		if err != nil {
			return engine.Row{}, err
		}
		// Skip offset rows.
		if n.skipped < n.offset {
			n.skipped++
			continue
		}
		// Emit up to limit rows.
		n.emitted++
		if n.limit > 0 && n.emitted > n.limit {
			return engine.Row{}, io.EOF
		}
		return row, nil
	}
}

func (n *LimitNode) Close() error  { return n.child.Close() }
func (n *LimitNode) Schema() engine.Schema { return n.outSchema.DeepCopy() }
