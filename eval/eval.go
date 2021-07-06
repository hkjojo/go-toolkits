package eval

import (
	"errors"

	"github.com/Knetic/govaluate"
	"github.com/fatih/structs"
	"github.com/hkjojo/go-toolkits/utils"
)

var ErrParameter = errors.New("parameter err")

func GetMatcher(expr string, test interface{}) (expression *govaluate.EvaluableExpression, err error) {
	functions := map[string]govaluate.ExpressionFunction{
		"match":   matchFunc,
		"any":     anyFunc,
		"range":   rangeFunc,
		"coloreq": colorEqFunc,
		"in":      inFunc,
	}
	expression, err = govaluate.NewEvaluableExpressionWithFunctions(expr, functions)
	if err != nil {
		return
	}
	ctx := structs.Map(test)
	// check var and type
	_, err = expression.Evaluate(ctx)
	if err != nil {
		return nil, err
	}
	// check var
	vars := expression.Vars()
	for _, s := range vars {
		if _, ok := ctx[s]; !ok {
			return nil, errors.New("No parameter '" + s + "' found.")
		}
	}
	return
}

func matchFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, ErrParameter
	}
	value, ok := args[0].(string)
	if !ok {
		return false, ErrParameter
	}
	pattern, ok := args[1].(string)
	if !ok {
		return false, ErrParameter
	}
	return utils.Match(value, pattern), nil
}

func anyFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, ErrParameter
	}
	value, ok := args[0].(string)
	if !ok {
		return false, ErrParameter
	}
	pattern, ok := args[1].(string)
	if !ok {
		return false, ErrParameter
	}
	return utils.Any(value, pattern), nil
}

func rangeFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 3 {
		return false, ErrParameter
	}
	value, ok := args[0].(string)
	if !ok {
		return false, ErrParameter
	}
	min, ok := args[1].(string)
	if !ok {
		return false, ErrParameter
	}
	max, ok := args[2].(string)
	if !ok {
		return false, ErrParameter
	}
	return value >= min && value < max, nil
}
func colorEqFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, ErrParameter
	}
	color, ok := args[0].(float64)
	if !ok {
		return false, ErrParameter
	}
	rgbName, ok := args[1].(string)
	if !ok {
		return false, ErrParameter
	}
	return utils.ColorEq(uint(color), rgbName)
}

func inFunc(args ...interface{}) (interface{}, error) {
	var arg float64

	if len(args) < 2 {
		return false, ErrParameter
	}

	for i, a := range args {
		a1, ok := a.(float64)
		if !ok {
			return false, ErrParameter
		}
		if i == 0 {
			arg = a1
			continue
		}

		if a1 == arg {
			return true, nil
		}
	}
	return false, nil
}
