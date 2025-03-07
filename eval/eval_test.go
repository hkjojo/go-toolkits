package eval

import (
	"testing"
)

func TestMatchIE(t *testing.T) {
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

func TestMatchMulti(t *testing.T) {
	type Test struct {
		Symbol string `structs:"symbol"`
	}
	e, _ := GenMatcherWithFuncs("matchMulti(symbol,'XAUUSD')", &Test{}, nil)
	b, err := e.Evaluate(map[string]interface{}{
		"symbol": "X,XAUUSD",
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Log(b)
}
func TestAnyMulti(t *testing.T) {
	type Test struct {
		Symbol string `structs:"symbol"`
	}
	e, _ := GenMatcherWithFuncs("anyMulti(symbol,'xauusd,XAUUSD')", &Test{}, nil)
	b, err := e.Evaluate(map[string]interface{}{
		"symbol": "X,XAUUSD",
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Log(b)
}
