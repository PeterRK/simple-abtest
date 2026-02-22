package core

import (
	"testing"
)

func TestEvalExprEqual(t *testing.T) {
	cfg := []byte(`[{"op":6,"dtype":1,"key":"country","s":"CN"}]`)
	nodes, err := ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"country": "CN"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"country": "US"}) {
		t.FailNow()
	}
}

func TestEvalExprIn(t *testing.T) {
	cfg := []byte(`[{"op":4,"dtype":1,"key":"bucket","ss":["A","B"]}]`)
	nodes, err := ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"bucket": "A"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"bucket": "C"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{}) {
		t.FailNow()
	}
}

func TestEvalExprEmpty(t *testing.T) {
	if !EvalExpr(nil, map[string]string{}) {
		t.FailNow()
	}
	if !EvalExpr([]ExprNode{}, map[string]string{}) {
		t.FailNow()
	}
}

func TestEvalExprIntCompare(t *testing.T) {
	cfg := []byte(`[{"op":10,"dtype":2,"key":"age","i":18}]`)
	nodes, err := ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"age": "20"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"age": "16"}) {
		t.FailNow()
	}
}

func TestEvalExprNotEqualAndNotIn(t *testing.T) {
	cfg := []byte(`[{"op":7,"dtype":1,"key":"os","s":"ios"}]`)
	nodes, err := ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"os": "android"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"os": "ios"}) {
		t.FailNow()
	}

	cfg = []byte(`[{"op":5,"dtype":1,"key":"seg","ss":["A","B"]}]`)
	nodes, err = ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"seg": "A"}) {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"seg": "C"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{}) {
		t.FailNow()
	}
}

func TestEvalExprFloatAndBounds(t *testing.T) {
	cfg := []byte(`[{"op":8,"dtype":3,"key":"ratio","f":1.5}]`)
	nodes, err := ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"ratio": "1.0"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"ratio": "2.0"}) {
		t.FailNow()
	}

	cfg = []byte(`[{"op":9,"dtype":2,"key":"score","i":10}]`)
	nodes, err = ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"score": "10"}) {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"score": "9"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"score": "11"}) {
		t.FailNow()
	}

	cfg = []byte(`[{"op":11,"dtype":2,"key":"level","i":3}]`)
	nodes, err = ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"level": "3"}) {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"level": "4"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"level": "2"}) {
		t.FailNow()
	}
}

func TestEvalExprMissingArg(t *testing.T) {
	cfg := []byte(`[{"op":8,"dtype":2,"key":"age","i":18}]`)
	nodes, err := ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{}) {
		t.FailNow()
	}
}

func TestEvalExprLogicAndOrNot(t *testing.T) {
	cfg := []byte(`[
{"op":1,"dtype":0,"l":1,"r":2},
{"op":6,"dtype":1,"key":"k1","s":"A"},
{"op":6,"dtype":1,"key":"k2","s":"B"}
]`)
	nodes, err := ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"k1": "A", "k2": "B"}) {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"k1": "A", "k2": "X"}) {
		t.FailNow()
	}

	cfg = []byte(`[
{"op":3,"dtype":0,"l":1},
{"op":6,"dtype":1,"key":"k","s":"X"}
]`)
	nodes, err = ParseExpr(cfg)
	if err != nil {
		t.FailNow()
	}
	if EvalExpr(nodes, map[string]string{"k": "X"}) {
		t.FailNow()
	}
	if !EvalExpr(nodes, map[string]string{"k": "Y"}) {
		t.FailNow()
	}
}

func TestParseExprBroken(t *testing.T) {
	cfg := []byte(`[{"op":6,"dtype":1,"key":"k","s":"v","l":1}]`)
	if _, err := ParseExpr(cfg); err == nil {
		t.FailNow()
	}
}
