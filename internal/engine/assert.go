package engine

// Assertions — declarative checks evaluated against the run result in a
// goja VM. Expression example: `res.status`, `res.body.orders.length`,
// `res.headers['content-type']`.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/friesipayung/rapi/internal/model"
)

const assertShim = `
var rapi = { response: __res, res: __res };
var res = __res;
`

// RunAssertions evaluates every enabled assertion against the result.
func RunAssertions(assertions []model.Assertion, res *model.RunResult) []model.AssertionResult {
	var out []model.AssertionResult
	for _, a := range assertions {
		if a.Disabled {
			continue
		}
		out = append(out, runAssertion(a, res))
	}
	return out
}

func runAssertion(a model.Assertion, res *model.RunResult) model.AssertionResult {
	r := model.AssertionResult{
		Expression: a.Expression,
		Operator:   a.Operator,
		Expected:   a.Value,
	}
	if strings.TrimSpace(a.Expression) == "" {
		r.Error = "empty expression"
		return r
	}
	vm := goja.New()
	vm.Set("__res", assertionResObj(res))
	if _, err := vm.RunString(assertShim); err != nil {
		r.Error = err.Error()
		return r
	}
	val, err := vm.RunString("(" + a.Expression + ")")
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Actual = fmt.Sprintf("%v", val.Export())
	r.Pass = compare(a.Operator, val.Export(), a.Value)
	return r
}

// compare applies the operator. Values compare as strings, except the numeric
// operators which coerce both sides to float.
func compare(op string, actual interface{}, expected string) bool {
	switch strings.ToLower(op) {
	case "exists":
		return actual != nil && fmt.Sprintf("%v", actual) != "undefined" && fmt.Sprintf("%v", actual) != "null"
	case "equal":
		return fmt.Sprintf("%v", actual) == expected
	case "notequal":
		return fmt.Sprintf("%v", actual) != expected
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), expected)
	case "matches":
		re, err := regexp.Compile(expected)
		return err == nil && re.MatchString(fmt.Sprintf("%v", actual))
	case "greaterthan":
		return toFloat(actual) > toFloat(expected)
	case "lessthan":
		return toFloat(actual) < toFloat(expected)
	default:
		return false
	}
}

func toFloat(v interface{}) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprintf("%v", v)), 64)
	return f
}

// assertionResObj builds the `res` object exposed to assertions.
func assertionResObj(res *model.RunResult) map[string]interface{} {
	if res == nil {
		return map[string]interface{}{}
	}
	headers := map[string]string{}
	for _, kv := range res.Headers {
		headers[strings.ToLower(kv.Name)] = kv.Value
	}
	return map[string]interface{}{
		"status":     res.Status,
		"statusText": res.StatusText,
		"timeMs":     res.TimeMs,
		"headers":    headers,
		"body":       parseJSONBody(res.Body),
	}
}
