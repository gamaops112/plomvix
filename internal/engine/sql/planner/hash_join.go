package planner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/plomvix/plomvix/internal/engine"
)

// ErrUnsupportedJoinKey is returned when a datum type cannot be hashed.
var ErrUnsupportedJoinKey = errors.New("planner: unsupported join key type")

// HashKey is a string representation of a datum for map lookup.
type HashKey string

// encodeHashKey converts a datum into a HashKey. Returns empty key for NULL values.
func encodeHashKey(d engine.Datum) (HashKey, error) {
	if d.Value == nil {
		return "", nil
	}
	switch d.Type {
	case engine.TypeInt64:
		return HashKey(fmt.Sprintf("i:%d", d.Value.(int64))), nil
	case engine.TypeUint64:
		return HashKey(fmt.Sprintf("u:%d", d.Value.(uint64))), nil
	case engine.TypeFloat64:
		return HashKey(fmt.Sprintf("f:%f", d.Value.(float64))), nil
	case engine.TypeBool:
		return HashKey(fmt.Sprintf("b:%t", d.Value.(bool))), nil
	case engine.TypeString:
		return HashKey("s:" + d.Value.(string)), nil
	default:
		return "", ErrUnsupportedJoinKey
	}
}

// HashJoinNode performs an in-memory hash join (equijoin only).
type HashJoinNode struct {
	left          Operator // probe
	right         Operator // build
	leftKey       int
	rightKey      int
	isLeftJoin    bool
	residual      BoundExpr
	plannerSchema PlannerSchema
	outSchema     engine.Schema
	logger        *slog.Logger

	hashTable   map[HashKey][]engine.Row
	buildTime   time.Duration
	rightClosed bool

	activeProbe   engine.Row
	activeMatches []engine.Row
	matchIdx      int
	matchedAny    bool

	buildRows      int
	probeRows      int
	outputRows     int
	probeTime      time.Duration
	estimatedBytes int64
}

// NewHashJoinNode creates a hash join operator.
func NewHashJoinNode(
	left, right Operator,
	leftKey, rightKey int,
	isLeftJoin bool,
	residual BoundExpr,
	logger *slog.Logger,
) *HashJoinNode {
	leftPS := GetPlannerSchema(left)
	rightPS := GetPlannerSchema(right)
	fields := make([]SchemaField, 0, len(leftPS.Fields)+len(rightPS.Fields))
	fields = append(fields, leftPS.Fields...)
	fields = append(fields, rightPS.Fields...)
	return &HashJoinNode{
		left:          left,
		right:         right,
		leftKey:       leftKey,
		rightKey:      rightKey,
		isLeftJoin:    isLeftJoin,
		residual:      residual,
		plannerSchema: PlannerSchema{Fields: fields},
		outSchema:     PlannerSchema{Fields: fields}.ToEngineSchema(),
		logger:        logger,
		hashTable:     make(map[HashKey][]engine.Row),
	}
}

func (n *HashJoinNode) PlannerSchema() PlannerSchema { return n.plannerSchema }

func (n *HashJoinNode) Open(ctx context.Context) error {
	if err := n.left.Open(ctx); err != nil {
		return err
	}
	n.rightClosed = false
	if err := n.right.Open(ctx); err != nil {
		_ = n.left.Close()
		return err
	}
	n.hashTable = make(map[HashKey][]engine.Row)
	n.buildRows = 0
	n.probeRows = 0
	n.outputRows = 0
	n.estimatedBytes = 0
	n.buildTime = 0
	n.probeTime = 0
	n.activeProbe = engine.Row{}
	n.activeMatches = nil
	n.matchIdx = 0
	n.matchedAny = false

	start := time.Now()
	for {
		row, err := n.right.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			_ = n.left.Close()
			_ = n.right.Close()
			n.rightClosed = true
			return err
		}
		keyVal, err := encodeHashKey(row.Datums[n.rightKey])
		if err != nil {
			_ = n.left.Close()
			_ = n.right.Close()
			n.rightClosed = true
			return err
		}
		if keyVal == "" {
			continue
		}
		n.hashTable[keyVal] = append(n.hashTable[keyVal], row.DeepCopy())
		n.buildRows++
		n.estimatedBytes += int64(len(row.Datums)) * 16
	}
	n.buildTime = time.Since(start)
	_ = n.right.Close()
	n.rightClosed = true
	return nil
}

func (n *HashJoinNode) Next(ctx context.Context) (engine.Row, error) {
	start := time.Now()
	defer func() { n.probeTime += time.Since(start) }()

	for {
		if n.matchIdx < len(n.activeMatches) {
			match := n.activeMatches[n.matchIdx]
			n.matchIdx++
			combined := combineRows(n.activeProbe, match)
			if n.residual != nil {
				d, err := n.residual.Eval(combined)
				if err != nil {
					return engine.Row{}, err
				}
				if b, ok := d.Value.(bool); ok && b {
					n.matchedAny = true
					n.outputRows++
					return combined, nil
				}
				continue
			}
			n.matchedAny = true
			n.outputRows++
			return combined, nil
		}

		if n.isLeftJoin && len(n.activeProbe.Datums) > 0 && !n.matchedAny {
			padded := nullPadRight(n.activeProbe, n.right.Schema())
			n.activeProbe = engine.Row{}
			n.outputRows++
			return padded, nil
		}

		pr, err := n.left.Next(ctx)
		if err != nil {
			return engine.Row{}, err
		}
		n.probeRows++
		n.activeProbe = pr.DeepCopy()
		n.matchedAny = false

		keyVal, err := encodeHashKey(pr.Datums[n.leftKey])
		if err != nil {
			return engine.Row{}, err
		}
		if keyVal == "" {
			n.activeMatches = nil
			n.matchIdx = 0
			continue
		}
		if matches, ok := n.hashTable[keyVal]; ok {
			n.activeMatches = matches
			n.matchIdx = 0
		} else {
			n.activeMatches = nil
			n.matchIdx = 0
		}
	}
}

func (n *HashJoinNode) Close() error {
	if n.logger != nil {
		n.logger.Info("joins: HashJoin metrics",
			slog.String("join_algorithm", "hash"),
			slog.Bool("left_outer", n.isLeftJoin),
			slog.Int("build_rows", n.buildRows),
			slog.Int("probe_rows", n.probeRows),
			slog.Int("output_rows", n.outputRows),
			slog.Duration("build_time_ms", n.buildTime),
			slog.Duration("probe_time_ms", n.probeTime),
			slog.Int64("estimated_memory_bytes", n.estimatedBytes),
		)
	}
	n.hashTable = nil
	n.activeProbe = engine.Row{}
	n.activeMatches = nil
	errL := n.left.Close()
	var errR error
	if !n.rightClosed {
		errR = n.right.Close()
		n.rightClosed = true
	}
	if errL != nil {
		return errL
	}
	return errR
}

func (n *HashJoinNode) Schema() engine.Schema { return n.outSchema.DeepCopy() }

// combineRows concatenates left and right datums.
func combineRows(left, right engine.Row) engine.Row {
	out := engine.Row{
		Datums: make([]engine.Datum, 0, len(left.Datums)+len(right.Datums)),
		RowID:  0,
	}
	out.Datums = append(out.Datums, left.Datums...)
	out.Datums = append(out.Datums, right.Datums...)
	return out
}

// nullPadRight returns a row with left datums + typed NULLs for right columns.
func nullPadRight(left engine.Row, rightSchema engine.Schema) engine.Row {
	out := engine.Row{
		Datums: make([]engine.Datum, 0, len(left.Datums)+len(rightSchema.Columns)),
		RowID:  0,
	}
	out.Datums = append(out.Datums, left.Datums...)
	for _, col := range rightSchema.Columns {
		out.Datums = append(out.Datums, engine.Datum{Type: col.Type, Value: nil})
	}
	return out
}
