package codegen

// Shell targets: cURL, HTTPie, and the raw HTTP request.

import (
	"fmt"
	"strings"
)

func init() {
	register(Target{Key: "curl", Label: "cURL", Syntax: "bash"}, genCurl)
	register(Target{Key: "httpie", Label: "HTTPie", Syntax: "bash"}, genHTTPie)
	register(Target{Key: "http", Label: "HTTP", Syntax: "http"}, genRawHTTP)
}

func shellQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", `'\'"'`) + "'"
}

func genCurl(s *Snippet) string {
	var b strings.Builder
	b.WriteString(headerComment("# ", s))
	b.WriteString("curl -X " + s.Method)
	for _, h := range s.Headers {
		b.WriteString(" \\\n  -H " + shellQuote(h.Name+": "+h.Value))
	}
	if s.Body != "" {
		b.WriteString(" \\\n  --data-raw " + shellQuote(s.Body))
	}
	if len(s.Form) > 0 {
		for _, f := range s.Form {
			b.WriteString(" \\\n  --data-urlencode " + shellQuote(f.Name+"="+f.Value))
		}
	}
	if len(s.Multipart) > 0 {
		for _, f := range s.Multipart {
			b.WriteString(" \\\n  -F " + shellQuote(f.Name+"="+f.Value))
		}
	}
	b.WriteString(" \\\n  " + shellQuote(effectiveURL(s)))
	if s.Timeout > 0 {
		b.WriteString(fmt.Sprintf(" \\\n  -m %d", s.Timeout))
	}
	b.WriteString("\n")
	return b.String()
}

func genHTTPie(s *Snippet) string {
	var b strings.Builder
	b.WriteString(headerComment("# ", s))
	method := ""
	if s.Method != "GET" {
		method = s.Method + " "
	}
	b.WriteString("http " + method + shellQuote(effectiveURL(s)))
	for _, h := range s.Headers {
		b.WriteString(" \\\n  " + shellQuote(h.Name+":"+h.Value))
	}
	if s.Body != "" {
		b.WriteString(" \\\n  --raw " + shellQuote(s.Body))
	}
	if len(s.Form) > 0 {
		for _, f := range s.Form {
			b.WriteString(" \\\n  " + shellQuote(f.Name+"="+f.Value))
		}
	}
	if len(s.Multipart) > 0 {
		for _, f := range s.Multipart {
			if strings.HasPrefix(f.Value, "@") {
				b.WriteString(" \\\n  " + shellQuote(f.Name+"@"+strings.TrimPrefix(f.Value, "@")))
			} else {
				b.WriteString(" \\\n  " + shellQuote(f.Name+"="+f.Value))
			}
		}
	}
	if s.Timeout > 0 {
		b.WriteString(fmt.Sprintf(" \\\n  --timeout %d", s.Timeout))
	}
	b.WriteString("\n")
	return b.String()
}

func genRawHTTP(s *Snippet) string {
	var b strings.Builder
	b.WriteString(headerComment("# ", s))
	u, err := parseURLParts(effectiveURL(s))
	host, path := "localhost", "/"
	if err == nil {
		host = u.host
		path = u.path + u.query
	}
	b.WriteString(s.Method + " " + path + " HTTP/1.1\n")
	b.WriteString("Host: " + host + "\n")
	for _, h := range s.Headers {
		b.WriteString(h.Name + ": " + h.Value + "\n")
	}
	if s.Body != "" {
		b.WriteString("Content-Length: " + fmt.Sprint(len(s.Body)) + "\n")
	}
	b.WriteString("\n")
	if s.Body != "" {
		b.WriteString(s.Body + "\n")
	}
	return b.String()
}

type urlParts struct {
	host  string
	path  string
	query string
}

func parseURLParts(raw string) (urlParts, error) {
	// minimal parse without net/url to keep {{vars}} intact
	var u urlParts
	rest := raw
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		u.host = rest[:i]
		rest = rest[i:]
	} else {
		u.host = rest
		rest = "/"
	}
	if i := strings.Index(rest, "?"); i >= 0 {
		u.path, u.query = rest[:i], rest[i:]
	} else {
		u.path = rest
	}
	if u.path == "" {
		u.path = "/"
	}
	// validate scheme presence crudely
	if !strings.Contains(raw, "://") {
		return u, fmt.Errorf("no scheme")
	}
	return u, nil
}

// sortKV placeholder removed — model.KV ordering is preserved as authored.
