package engine

// Scripts (goja) — pre/post hooks with a rAPI-specific API surface.
//
// Pre-script API:  rapi.request (method/url/headers), rapi.env.get/set,
//                  rapi.vars.get/set, console.log
// Post-script API: everything above plus rapi.response
//                  (status/statusText/headers/body as parsed JSON)

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/friesipayung/rapi/internal/model"
)

// ScriptResult carries what a script changed.
type ScriptResult struct {
	Vars    Vars     // vars set via rapi.vars.set / rapi.env.set
	Console []string // console.log output
	Error   string   // script error, empty on success
}

const preScriptShim = `
var rapi = {
  request: __req,
  env: {
    get: function (k) { return __vars[k]; },
    set: function (k, v) { __out.vars[k] = String(v); }
  },
  vars: {
    get: function (k) { return __vars[k]; },
    set: function (k, v) { __out.vars[k] = String(v); }
  },
  response: __res,
  console: __console
};
var console = __console;
`

// RunScript executes a pre or post script in a fresh goja VM.
func RunScript(script model.Script, req *model.HttpRequest, res *model.RunResult, vars Vars) *ScriptResult {
	out := &ScriptResult{Vars: Vars{}}
	if strings.TrimSpace(script.Content) == "" {
		return out
	}
	vm := goja.New()

	// __out: mutation target
	var console []string
	vm.Set("__out", map[string]interface{}{"vars": map[string]interface{}{}})

	// __vars: read-only view of the merged scope
	view := map[string]interface{}{}
	for k, v := range vars {
		view[k] = v
	}
	vm.Set("__vars", view)

	// __req: request snapshot for the script
	vm.Set("__req", map[string]interface{}{
		"method": req.Method,
		"url":    req.URL,
		"headers": func() map[string]string {
			h := map[string]string{}
			for _, kv := range EnabledKV(req.Headers) {
				h[kv.Name] = kv.Value
			}
			return h
		}(),
	})

	// __res: response for post scripts (body parsed when JSON)
	var resObj interface{}
	if res != nil {
		resObj = map[string]interface{}{
			"status":     res.Status,
			"statusText": res.StatusText,
			"timeMs":     res.TimeMs,
			"headers": func() map[string]string {
				h := map[string]string{}
				for _, kv := range res.Headers {
					h[strings.ToLower(kv.Name)] = kv.Value
				}
				return h
			}(),
			"body": parseJSONBody(res.Body),
		}
	}
	vm.Set("__res", resObj)

	vm.Set("__console", map[string]interface{}{
		"log": func(args ...interface{}) {
			parts := make([]string, len(args))
			for i, a := range args {
				parts[i] = fmt.Sprintf("%v", a)
			}
			console = append(console, strings.Join(parts, " "))
		},
	})

	if _, err := vm.RunString(preScriptShim + script.Content); err != nil {
		out.Error = err.Error()
		out.Console = console
		return out
	}

	// collect __out.vars
	raw, err := json.Marshal(vm.Get("__out").Export())
	if err == nil {
		var m struct {
			Vars map[string]interface{} `json:"vars"`
		}
		if json.Unmarshal(raw, &m) == nil {
			for k, v := range m.Vars {
				out.Vars[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	out.Console = console
	return out
}

// parseJSONBody returns the response body as a goja-friendly value: parsed
// JSON when possible, otherwise the raw string.
func parseJSONBody(rb *model.ResponseBody) interface{} {
	if rb == nil {
		return nil
	}
	if rb.Type != "json" {
		if rb.Type == "binary" {
			return "(binary)"
		}
		return rb.Data
	}
	var v interface{}
	if err := json.Unmarshal([]byte(rb.Data), &v); err != nil {
		return rb.Data
	}
	return v
}
