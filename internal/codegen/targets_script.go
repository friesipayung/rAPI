package codegen

// Python (requests), JavaScript (fetch), Node.js (native http + axios).

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	register(Target{Key: "python", Label: "Python · requests", Syntax: "python"}, genPython)
	register(Target{Key: "js-fetch", Label: "JavaScript · fetch", Syntax: "javascript"}, genFetch)
	register(Target{Key: "node-native", Label: "Node.js · native", Syntax: "javascript"}, genNodeNative)
	register(Target{Key: "node-axios", Label: "Node.js · axios", Syntax: "javascript"}, genAxios)
}

// ---------- Python ----------

func pyStr(v string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n").Replace(v) + "'"
}

func genPython(s *Snippet) string {
	var b strings.Builder
	b.WriteString(headerComment("# ", s))
	b.WriteString("import requests\n\n")
	b.WriteString("url = " + pyStr(effectiveURL(s)) + "\n")
	if len(s.Headers) > 0 {
		b.WriteString("headers = {\n")
		for _, h := range s.Headers {
			b.WriteString("    " + pyStr(h.Name) + ": " + pyStr(h.Value) + ",\n")
		}
		b.WriteString("}\n")
	}
	payload := "None"
	if s.Body != "" {
		payload = pyStr(s.Body)
	} else if len(s.Form) > 0 {
		b.WriteString("data = {\n")
		for _, f := range s.Form {
			b.WriteString("    " + pyStr(f.Name) + ": " + pyStr(f.Value) + ",\n")
		}
		b.WriteString("}\n")
		payload = "data"
	} else if len(s.Multipart) > 0 {
		b.WriteString("files = {\n")
		for _, f := range s.Multipart {
			if strings.HasPrefix(f.Value, "@") {
				b.WriteString("    " + pyStr(f.Name) + ": open(" + pyStr(strings.TrimPrefix(f.Value, "@")) + ", 'rb'),\n")
			} else {
				b.WriteString("    " + pyStr(f.Name) + ": (None, " + pyStr(f.Value) + "),\n")
			}
		}
		b.WriteString("}\n")
		payload = "files"
	}
	args := "url"
	if len(s.Headers) > 0 {
		args += ", headers=headers"
	}
	if payload != "None" {
		args += ", " + (map[bool]string{true: "data", false: "json"})[payload == "data" || payload == "files"]
		args += "=" + payload
	}
	if s.Timeout > 0 {
		args += fmt.Sprintf(", timeout=%d", s.Timeout)
	}
	if s.Follow != nil && !*s.Follow {
		args += ", allow_redirects=False"
	}
	b.WriteString("response = requests." + strings.ToLower(s.Method) + "(" + args + ")\n")
	b.WriteString("print(response.status_code)\n")
	b.WriteString("print(response.text)\n")
	return b.String()
}

// ---------- JavaScript fetch ----------

func jsStr(v string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n").Replace(v) + "'"
}

func genFetch(s *Snippet) string {
	var b strings.Builder
	b.WriteString(headerComment("// ", s))
	b.WriteString("const url = " + jsStr(effectiveURL(s)) + ";\n\n")
	b.WriteString("const options = {\n")
	b.WriteString("  method: " + jsStr(s.Method) + ",\n")
	if len(s.Headers) > 0 {
		b.WriteString("  headers: {\n")
		for _, h := range s.Headers {
			b.WriteString("    " + jsStr(h.Name) + ": " + jsStr(h.Value) + ",\n")
		}
		b.WriteString("  },\n")
	}
	if s.Body != "" {
		b.WriteString("  body: " + jsStr(s.Body) + ",\n")
	} else if len(s.Form) > 0 {
		b.WriteString("  body: new URLSearchParams({\n")
		for _, f := range s.Form {
			b.WriteString("    " + jsStr(f.Name) + ": " + jsStr(f.Value) + ",\n")
		}
		b.WriteString("  }),\n")
	} else if len(s.Multipart) > 0 {
		b.WriteString("  // FormData: browser / Node 18+\n")
		b.WriteString("  body: (() => { const fd = new FormData();\n")
		for _, f := range s.Multipart {
			if strings.HasPrefix(f.Value, "@") {
				b.WriteString("    fd.append(" + jsStr(f.Name) + ", fileInput.files[0]); // " + strings.TrimPrefix(f.Value, "@") + "\n")
			} else {
				b.WriteString("    fd.append(" + jsStr(f.Name) + ", " + jsStr(f.Value) + ");\n")
			}
		}
		b.WriteString("    return fd; })(),\n")
	}
	b.WriteString("};\n\n")
	b.WriteString("fetch(url, options)\n")
	b.WriteString("  .then((res) => res.text())\n")
	b.WriteString("  .then((body) => console.log(body));\n")
	return b.String()
}

// ---------- Node.js native ----------

func genNodeNative(s *Snippet) string {
	var b strings.Builder
	b.WriteString(headerComment("// ", s))
	b.WriteString("const http = require('http');\n")
	b.WriteString("const https = require('https');\n\n")
	b.WriteString("const url = new URL(" + jsStr(effectiveURL(s)) + ");\n\n")
	b.WriteString("const options = {\n")
	b.WriteString("  method: " + jsStr(s.Method) + ",\n")
	b.WriteString("  hostname: url.hostname,\n")
	b.WriteString("  port: url.port || (url.protocol === 'https:' ? 443 : 80),\n")
	b.WriteString("  path: url.pathname + url.search,\n")
	if len(s.Headers) > 0 {
		b.WriteString("  headers: {\n")
		for _, h := range s.Headers {
			b.WriteString("    " + jsStr(h.Name) + ": " + jsStr(h.Value) + ",\n")
		}
		b.WriteString("  },\n")
	}
	b.WriteString("};\n\n")
	b.WriteString("const req = (url.protocol === 'https:' ? https : http).request(options, (res) => {\n")
	b.WriteString("  let body = '';\n")
	b.WriteString("  res.on('data', (chunk) => (body += chunk));\n")
	b.WriteString("  res.on('end', () => console.log(res.statusCode, body));\n")
	b.WriteString("});\n")
	if s.Body != "" {
		b.WriteString("req.write(" + jsStr(s.Body) + ");\n")
	} else if len(s.Form) > 0 {
		parts := make([]string, 0, len(s.Form))
		for _, f := range s.Form {
			parts = append(parts, f.Name+"="+f.Value)
		}
		b.WriteString("req.write(" + jsStr(strings.Join(parts, "&")) + ");\n")
	}
	b.WriteString("req.end();\n")
	return b.String()
}

// ---------- Node.js axios ----------

func genAxios(s *Snippet) string {
	var b strings.Builder
	b.WriteString(headerComment("// ", s))
	b.WriteString("const axios = require('axios');\n\n")
	b.WriteString("const config = {\n")
	b.WriteString("  method: " + jsStr(strings.ToLower(s.Method)) + ",\n")
	b.WriteString("  url: " + jsStr(effectiveURL(s)) + ",\n")
	if len(s.Headers) > 0 {
		b.WriteString("  headers: {\n")
		for _, h := range s.Headers {
			b.WriteString("    " + jsStr(h.Name) + ": " + jsStr(h.Value) + ",\n")
		}
		b.WriteString("  },\n")
	}
	if s.Body != "" {
		var pretty string
		if json.Unmarshal([]byte(s.Body), new(interface{})) == nil {
			pretty = s.Body // keep as object literal
			b.WriteString("  data: " + compactJSONLiteral(pretty) + ",\n")
		} else {
			b.WriteString("  data: " + jsStr(s.Body) + ",\n")
		}
	} else if len(s.Form) > 0 {
		b.WriteString("  data: new URLSearchParams({\n")
		for _, f := range s.Form {
			b.WriteString("    " + jsStr(f.Name) + ": " + jsStr(f.Value) + ",\n")
		}
		b.WriteString("  }),\n")
	} else if len(s.Multipart) > 0 {
		b.WriteString("  data: (() => { const fd = new FormData();\n")
		for _, f := range s.Multipart {
			if strings.HasPrefix(f.Value, "@") {
				b.WriteString("    fd.append(" + jsStr(f.Name) + ", fs.createReadStream(" + jsStr(strings.TrimPrefix(f.Value, "@")) + "));\n")
			} else {
				b.WriteString("    fd.append(" + jsStr(f.Name) + ", " + jsStr(f.Value) + ");\n")
			}
		}
		b.WriteString("    return fd; })(),\n")
	}
	if s.Timeout > 0 {
		b.WriteString(fmt.Sprintf("  timeout: %d,\n", s.Timeout*1000))
	}
	b.WriteString("};\n\n")
	b.WriteString("axios(config).then((res) => console.log(res.status, res.data));\n")
	return b.String()
}

// compactJSONLiteral re-emits a JSON body as a JS object literal (best effort).
func compactJSONLiteral(s string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return jsStr(s)
	}
	out, err := marshalJS(v)
	if err != nil {
		return jsStr(s)
	}
	return out
}

// marshalJS renders a decoded JSON value as a JS literal.
func marshalJS(v interface{}) (string, error) {
	// JSON is a subset of JS literals, so json.Marshal output is valid JS.
	b, err := json.Marshal(v)
	return string(b), err
}
