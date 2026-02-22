package core

import (
	"errors"
	"strconv"

	json "github.com/goccy/go-json"
	"github.com/peterrk/simple-abtest/utils"
)

// OpType represents the logical or comparison operation used in an expression node.
type OpType int

// DataType describes the type of value an expression node operates on.
type DataType int

const (
	OpNull OpType = iota
	OpAnd
	OpOr
	OpNot
	OpIn
	OpNotIn
	OpEqual
	OpNotEqual
	OpLessThan
	OpLessOrEqual
	OpGreatThan
	OpGreatOrEqual
)

const (
	DtNull DataType = iota
	DtStr
	DtInt
	DtFloat
)

// ExprNode describes a single node in a boolean expression tree used
// to filter experiments based on request context.
// The tree is constructed from a flat slice using Left and Right indices.
type ExprNode struct {
	Op      OpType   `json:"op"`
	DType   DataType `json:"dtype,omitempty"`
	Key     string   `json:"key,omitempty"`
	ParamS  string   `json:"s,omitempty"`
	ParamI  int64    `json:"i,omitempty"`
	ParamF  float64  `json:"f,omitempty"`
	ParamSS []string `json:"ss,omitempty"`
	Left    uint     `json:"l,omitempty"`
	Right   uint     `json:"r,omitempty"`

	ss    map[string]bool
	left  *ExprNode
	right *ExprNode
}

var (
	errBrokenConfig = errors.New("broken config")
	errArgmentMiss  = errors.New("argment miss")
)

// ParseExpr parses a JSON encoded expression configuration into a slice
// of ExprNode and wires their child pointers. It validates the structure
// and returns errBrokenConfig on invalid input.
func ParseExpr(config []byte) ([]ExprNode, error) {
	if len(config) == 0 {
		return nil, nil
	}
	var nodes []ExprNode
	err := json.Unmarshal(config, &nodes)
	if err != nil {
		return nil, err
	}

	used := make([]bool, len(nodes))
	for i := 0; i < len(nodes); i++ {
		node := &nodes[i]
		if node.Left >= uint(len(nodes)) || node.Right >= uint(len(nodes)) ||
			used[node.Left] || used[node.Right] {
			return nil, errBrokenConfig
		}
		if node.Left > 0 {
			node.left = &nodes[node.Left]
			used[node.Left] = true
		}
		if node.Right > 0 {
			node.right = &nodes[node.Right]
			used[node.Right] = true
		}

		switch node.Op {
		case OpAnd, OpOr:
			if node.left == nil || node.right == nil || node.DType != DtNull {
				return nil, errBrokenConfig
			}
		case OpNot:
			if node.left == nil || node.right != nil || node.DType != DtNull {
				return nil, errBrokenConfig
			}
		case OpIn, OpNotIn:
			if node.left != nil || node.right != nil || len(node.Key) == 0 ||
				node.DType != DtStr || len(node.ParamSS) == 0 {
				return nil, errBrokenConfig
			}
			node.ss = utils.ListToSet(node.ParamSS)
		case OpEqual, OpNotEqual, OpLessThan, OpLessOrEqual, OpGreatThan, OpGreatOrEqual:
			if node.left != nil || node.right != nil || len(node.Key) == 0 ||
				(node.DType != DtStr && node.DType != DtInt && node.DType != DtFloat) {
				return nil, errBrokenConfig
			}
		default:
			return nil, errBrokenConfig
		}
	}
	return nodes, nil
}

type ordered interface {
	string | int64 | float64
}

func cmpEqual[T ordered](a, b T) bool {
	return a == b
}

func cmpNotEqual[T ordered](a, b T) bool {
	return a != b
}

func cmpLessThan[T ordered](a, b T) bool {
	return a < b
}

func cmpLessOrEqual[T ordered](a, b T) bool {
	return a <= b
}

func cmpGreatThan[T ordered](a, b T) bool {
	return a > b
}

func cmpGreatOrEqual[T ordered](a, b T) bool {
	return a >= b
}

// EvalExpr evaluates a parsed expression using the provided argument map.
// It returns true when the expression passes; missing or malformed arguments
// cause evaluation to fail and return false.
func EvalExpr(expr []ExprNode, args map[string]string) bool {
	if len(expr) == 0 {
		return true
	}

	cmpOp := func(node *ExprNode, fs func(a, b string) bool,
		fi func(a, b int64) bool, ff func(a, b float64) bool) (bool, error) {
		str, got := args[node.Key]
		if !got {
			return false, errArgmentMiss
		}
		switch node.DType {
		case DtStr:
			return fs(str, node.ParamS), nil
		case DtInt:
			if val, err := strconv.ParseInt(str, 10, 64); err != nil {
				return false, err
			} else {
				return fi(val, node.ParamI), nil
			}
		case DtFloat:
			if val, err := strconv.ParseFloat(str, 64); err != nil {
				return false, err
			} else {
				return ff(val, node.ParamF), nil
			}
		}
		return false, errBrokenConfig
	}

	var eval func(*ExprNode) (bool, error)
	eval = func(node *ExprNode) (bool, error) {
		switch node.Op {
		case OpAnd:
			if pass, err := eval(node.left); err != nil {
				return false, err
			} else if !pass {
				return false, nil
			}
			if pass, err := eval(node.right); err != nil {
				return false, err
			} else {
				return pass, nil
			}
		case OpOr:
			if pass, err := eval(node.left); err != nil {
				return false, err
			} else if pass {
				return true, nil
			}
			if pass, err := eval(node.right); err != nil {
				return false, err
			} else {
				return pass, nil
			}
		case OpNot:
			if pass, err := eval(node.left); err != nil {
				return false, err
			} else {
				return !pass, nil
			}
		case OpIn:
			val, got := args[node.Key]
			if !got {
				return false, errArgmentMiss
			}
			return node.ss[val], nil
		case OpNotIn:
			val, got := args[node.Key]
			if !got {
				return false, errArgmentMiss
			}
			return !node.ss[val], nil

		case OpEqual:
			return cmpOp(node, cmpEqual[string], cmpEqual[int64], cmpEqual[float64])
		case OpNotEqual:
			return cmpOp(node, cmpNotEqual[string], cmpNotEqual[int64], cmpNotEqual[float64])
		case OpLessThan:
			return cmpOp(node, cmpLessThan[string], cmpLessThan[int64], cmpLessThan[float64])
		case OpLessOrEqual:
			return cmpOp(node, cmpLessOrEqual[string], cmpLessOrEqual[int64], cmpLessOrEqual[float64])
		case OpGreatThan:
			return cmpOp(node, cmpGreatThan[string], cmpGreatThan[int64], cmpGreatThan[float64])
		case OpGreatOrEqual:
			return cmpOp(node, cmpGreatOrEqual[string], cmpGreatOrEqual[int64], cmpGreatOrEqual[float64])
		}
		return false, errBrokenConfig
	}

	pass, err := eval(&expr[0])
	return err == nil && pass
}
