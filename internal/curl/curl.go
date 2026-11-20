package curl

// Parse a pasted curl or wget command line into a model.HttpRequest.
// Hand-rolled shell tokenizer (single/double quotes, backslash escapes,
// backslash-newline continuations) — no external dependency.

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
)

// Parse detects curl vs wget and converts the command.
func Parse(cmd string) (*model.HttpRequest, error) {
	argv, err := Tokenize(cmd)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	base := strings.TrimSpace(argv[0])
	base = strings.ToLower(base)
	switch {
	case strings.HasSuffix(base, "curl"):
		return parseCurl(argv)
	case strings.HasSuffix(base, "wget"):
		return parseWget(argv)
	default:
		return nil, fmt.Errorf("not a curl or wget command (got %q)", argv[0])
	}
}

// Tokenize splits a shell-like command into words.
func Tokenize(cmd string) ([]string, error) {
	var out []string
	var cur strings.Builder
	i := 0
	n := len(cmd)
	inWord := false
	flush := func() {
		if inWord {
			out = append(out, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	for i < n {
		c := cmd[i]
		switch {
		case c == '\\' && i+1 < n:
			// backslash escape: outside quotes it also eats line breaks
			if cmd[i+1] == '\n' {
				i += 2
				continue
			}
			cur.WriteByte(cmd[i+1])
			i += 2
			inWord = true
		case c == '\'':
			inWord = true
			i++
			for i < n && cmd[i] != '\'' {
				cur.WriteByte(cmd[i])
				i++
			}
			if i >= n {
				return nil, fmt.Errorf("unterminated single quote")
			}
			i++ // closing quote
		case c == '"':
			inWord = true
			i++
			for i < n && cmd[i] != '"' {
				if cmd[i] == '\\' && i+1 < n && (cmd[i+1] == '"' || cmd[i+1] == '\\' || cmd[i+1] == '$' || cmd[i+1] == '`') {
					cur.WriteByte(cmd[i+1])
					i += 2
					continue
				}
				cur.WriteByte(cmd[i])
				i++
			}
			if i >= n {
				return nil, fmt.Errorf("unterminated double quote")
			}
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
			i++
		default:
			cur.WriteByte(c)
			inWord = true
			i++
		}
	}
	flush()
	return out, nil
}

type curlOpts struct {
	method    string
	headers   []model.KV
	dataParts []string // -d style payloads, joined with &
	formParts []string // -F style payloads
	url       string
	user      string
	bearer    string
	get       bool // -G: send data as query
	head      bool
	timeout   int
}

// valueFlags: long flags that take a separate or '='-attached value.
var valueFlags = map[string]bool{
	"--request": true, "--header": true, "--data": true, "--data-raw": true,
	"--data-binary": true, "--data-ascii": true, "--data-urlencode": true,
	"--form": true, "--form-string": true, "--url": true, "--user": true,
	"--oauth2-bearer": true, "--max-time": true, "--user-agent": true,
	"--referer": true, "--cookie": true, "--json": true, "--connect-timeout": true,
}

// shortValue: single-letter flags that take a value (separate or attached).
var shortValue = map[string]bool{
	"X": true, "H": true, "d": true, "F": true, "u": true, "m": true,
	"A": true, "e": true, "b": true, "o": true, "T": true, "--": false,
}

func parseCurl(argv []string) (*model.HttpRequest, error) {
	var o curlOpts
	i := 1
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--":
			i++
			if i < len(argv) && o.url == "" {
				o.url = argv[i]
				i++
			}
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := splitLong(a)
			if !valueFlags[name] {
				// ignore (boolean or unknown flags: -L handled below)
				if name == "--location" || name == "--get" || name == "--head" {
					// handled as boolean
				}
				if hasVal || name == "--location" || name == "--get" || name == "--head" {
					// fallthrough to consume nothing extra
				}
				i++
				continue
			}
			if !hasVal {
				if i+1 >= len(argv) {
					return nil, fmt.Errorf("missing value for %s", name)
				}
				i++
				val = argv[i]
			}
			applyCurlValue(&o, name, val)
			i++
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// short flag(s)
			flag := a[1:2]
			rest := a[2:]
			switch flag {
			case "X":
				if rest != "" {
					o.method = rest
				} else if i+1 < len(argv) {
					i++
					o.method = argv[i]
				}
			case "H", "d", "F", "u", "A", "e", "b", "m":
				var v string
				if rest != "" {
					v = rest
				} else if i+1 < len(argv) {
					i++
					v = argv[i]
				}
				applyShortValue(&o, flag, v)
			case "G":
				o.get = true
			case "I":
				o.head = true
			case "L":
				// follow redirects: handled by caller-side settings default
			default:
				// -s, -v, -k, -i, -o (with attached rest consumed), etc.
				if rest == "" && !shortValue[flag] {
					// boolean cluster like -skL — nothing to do
				}
			}
			i++
		default:
			if o.url == "" {
				o.url = a
			}
			i++
		}
	}
	return build(&o)
}

func splitLong(a string) (name, val string, has bool) {
	if idx := strings.Index(a, "="); idx >= 0 {
		return a[:idx], a[idx+1:], true
	}
	return a, "", false
}

func applyCurlValue(o *curlOpts, name, val string) {
	switch name {
	case "--request":
		o.method = strings.ToUpper(val)
	case "--header":
		o.headers = append(o.headers, parseHeader(val))
	case "--user-agent":
		o.headers = append(o.headers, model.KV{Name: "User-Agent", Value: val})
	case "--referer":
		o.headers = append(o.headers, model.KV{Name: "Referer", Value: val})
	case "--cookie":
		o.headers = append(o.headers, model.KV{Name: "Cookie", Value: val})
	case "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode", "--json":
		o.dataParts = append(o.dataParts, val)
		if name == "--json" {
			o.headers = append(o.headers, model.KV{Name: "Content-Type", Value: "application/json"})
			o.headers = append(o.headers, model.KV{Name: "Accept", Value: "application/json"})
		}
	case "--form", "--form-string":
		o.formParts = append(o.formParts, val)
	case "--url":
		if o.url == "" {
			o.url = val
		}
	case "--user":
		o.user = val
	case "--oauth2-bearer":
		o.bearer = val
	case "--max-time":
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			o.timeout = n
		}
	}
}

func applyShortValue(o *curlOpts, flag, v string) {
	switch flag {
	case "H":
		o.headers = append(o.headers, parseHeader(v))
	case "d":
		o.dataParts = append(o.dataParts, v)
	case "F":
		o.formParts = append(o.formParts, v)
	case "u":
		o.user = v
	case "A":
		o.headers = append(o.headers, model.KV{Name: "User-Agent", Value: v})
	case "e":
		o.headers = append(o.headers, model.KV{Name: "Referer", Value: v})
	case "b":
		o.headers = append(o.headers, model.KV{Name: "Cookie", Value: v})
	case "m":
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			o.timeout = n
		}
	}
}

func parseHeader(h string) model.KV {
	idx := strings.Index(h, ":")
	if idx < 0 {
		return model.KV{Name: strings.TrimSpace(h), Value: ""}
	}
	return model.KV{Name: strings.TrimSpace(h[:idx]), Value: strings.TrimSpace(h[idx+1:])}
}

func build(o *curlOpts) (*model.HttpRequest, error) {
	if o.url == "" {
		return nil, fmt.Errorf("no URL found in command")
	}
	req := &model.HttpRequest{Method: "GET", URL: o.url, Headers: o.headers}

	// auth
	if o.bearer != "" {
		req.Auth = &model.Auth{Type: model.AuthBearer, Bearer: &model.BearerAuth{Token: o.bearer}}
	} else if o.user != "" {
		u, p, _ := strings.Cut(o.user, ":")
		req.Auth = &model.Auth{Type: model.AuthBasic, Basic: &model.BasicAuth{Username: u, Password: p}}
	}

	// method + body
	data := strings.Join(o.dataParts, "&")
	if len(o.formParts) > 0 {
		req.Method = "POST"
		req.Body = &model.Body{Type: model.BodyMultipart, FormData: parseFormFields(o.formParts)}
	} else if data != "" {
		if o.get {
			req.Method = "GET"
		} else if o.method == "" {
			req.Method = "POST"
		} else {
			req.Method = o.method
		}
		if o.get {
			// -G: data rides in the query string
			req.Params = append(req.Params, parseQueryIntoParams(data)...)
		} else if isJSON(data) {
			req.Body = &model.Body{Type: model.BodyJSON, Data: data}
		} else {
			// curl's -d default content type
			if !hasHeader(req.Headers, "Content-Type") {
				req.Headers = append(req.Headers, model.KV{Name: "Content-Type", Value: "application/x-www-form-urlencoded"})
			}
			req.Body = &model.Body{Type: model.BodyForm, FormData: parseFormFields(strings.Split(data, "&"))}
		}
	} else if o.method != "" {
		req.Method = o.method
	}
	if o.head {
		req.Method = "HEAD"
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	// drop a multipart/bearer note: keep headers verbatim
	return req, nil
}

func isJSON(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")
}

func hasHeader(hs []model.KV, name string) bool {
	for _, h := range hs {
		if strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}

// parseFormFields splits "a=1&b=2" (or -F "a=1") entries into KV pairs.
// Values stay raw; rAPI's engine encodes when sending.
func parseFormFields(parts []string) []model.KV {
	var out []model.KV
	for _, p := range parts {
		if p == "" {
			continue
		}
		k, v, _ := strings.Cut(p, "=")
		out = append(out, model.KV{Name: k, Value: v})
	}
	return out
}

// parseQueryIntoParams splits "a=1&b=2" into request params.
func parseQueryIntoParams(data string) []model.KV {
	var out []model.KV
	for _, kv := range parseFormFields(strings.Split(data, "&")) {
		k, _ := url.QueryUnescape(kv.Name)
		v, _ := url.QueryUnescape(kv.Value)
		if k == "" {
			continue
		}
		out = append(out, model.KV{Name: k, Value: v, Type: "query"})
	}
	return out
}

// ---------- wget ----------

type wgetOpts struct {
	method  string
	headers []model.KV
	body    string
	url     string
	timeout int
}

var wgetValueFlags = map[string]bool{
	"--method": true, "--header": true, "--body-data": true, "--body-content": true,
	"--post-data": true, "--body-file": true, "--post-file": true, "--timeout": true,
	"--user-agent": true, "--header-file": true,
}

func parseWget(argv []string) (*model.HttpRequest, error) {
	var o wgetOpts
	i := 1
	for i < len(argv) {
		a := argv[i]
		switch {
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := splitLong(a)
			if !wgetValueFlags[name] {
				i++
				continue
			}
			if !hasVal {
				if i+1 >= len(argv) {
					return nil, fmt.Errorf("missing value for %s", name)
				}
				i++
				val = argv[i]
			}
			switch name {
			case "--method":
				o.method = strings.ToUpper(val)
			case "--header":
				o.headers = append(o.headers, parseHeader(val))
			case "--user-agent":
				o.headers = append(o.headers, model.KV{Name: "User-Agent", Value: val})
			case "--body-data", "--body-content", "--post-data":
				o.body = val
				if o.method == "" {
					o.method = "POST"
				}
			case "--body-file", "--post-file":
				// file contents are not available at parse time; reference it
				o.body = ""
				o.headers = append(o.headers, model.KV{Name: "X-Wget-Body-File", Value: val})
				if o.method == "" {
					o.method = "POST"
				}
			case "--timeout":
				if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
					o.timeout = n
				}
			}
			i++
		case strings.HasPrefix(a, "-") && len(a) > 1:
			switch a[1] {
			case 'U': // user agent
				var v string
				if len(a) > 2 {
					v = a[2:]
				} else if i+1 < len(argv) {
					i++
					v = argv[i]
				}
				o.headers = append(o.headers, model.KV{Name: "User-Agent", Value: v})
			case 'T':
				var v string
				if len(a) > 2 {
					v = a[2:]
				} else if i+1 < len(argv) {
					i++
					v = argv[i]
				}
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					o.timeout = n
				}
			default:
				// -q, -O, -c, --no-check-certificate … ignored
			}
			i++
		default:
			if o.url == "" {
				o.url = a
			}
			i++
		}
	}
	if o.url == "" {
		return nil, fmt.Errorf("no URL found in command")
	}
	req := &model.HttpRequest{Method: "GET", URL: o.url, Headers: o.headers}
	if o.method != "" {
		req.Method = o.method
	}
	if o.body != "" {
		if isJSON(o.body) {
			req.Body = &model.Body{Type: model.BodyJSON, Data: o.body}
		} else {
			req.Body = &model.Body{Type: model.BodyText, Data: o.body}
		}
		if req.Method == "GET" {
			req.Method = "POST"
		}
	}
	return req, nil
}
