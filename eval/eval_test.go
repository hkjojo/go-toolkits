package eval

import (
	"testing"
)

func TestEval(t *testing.T) {
	type Test struct {
		Symbol string `structs:"symbol"`
	}
	e, _ := GenMatcherWithFuncs("matchIE(symbol,'XauUSD')", &Test{}, nil)
	b, err := e.Evaluate(map[string]interface{}{
		"symbol": "XAUUSD",
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Log(b)
}
