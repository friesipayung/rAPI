package codegen

// Snippet: the intermediate representation all emitters consume, built once
// from a model.HttpRequest. Emitters never touch the model directly.

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
)

// Target describes one generator for the UI.
type Target struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Syntax string `json:"syntax"` // highlight hint for the frontend
}

type Snippet struct {
	Method    string
	URL       string            // base URL, query params NOT appended
	Query     []model.KV        // effective query parameters
	Headers   []model.KV        // effective headers (incl. auth headers where not native)
	Body      string            // raw body (json/text/xml/html)
	Form      []model.KV        // form-urlencoded fields
	Multipart []model.KV        // multipart fields (value may start with @)
	HasFiles  bool              // any multipart value starts with @
	AuthNote  string            // extra comment lines about auth
	Vars      []string          // {{placeholders}} found, sorted+unique
	Timeout   int               // seconds; 0 = unset
	Follow    *bool             // nil = library default
}

var varRe = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

func collectVars(parts ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range parts {
		for _, m := range varRe.FindAllStringSubmatch(p, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	sort.Strings(out)
	return out
}

// Build converts a request into a Snippet.
func Build(req *model.HttpRequest, settings *model.Settings) (*Snippet, error) {
	if req == nil {
		return nil, fmt.Errorf("no request")
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("request has no URL")
	}
	s := &Snippet{Method: strings.ToUpper(req.Method), URL: rawURL}
	if s.Method == "" {
		s.Method = "GET"
	}

	// split query params off the URL
	if u, err := url.Parse(rawURL); err == nil && u.RawQuery != "" {
		s.URL = strings.TrimSuffix(rawURL, "?"+u.RawQuery)
		for _, kv := range u.Query() {
			for _, v := range kv {
				s.Query = append(s.Query, model.KV{Name: kv[0], Value: v})
			}
		}
	}
	// request params → query
	s.Query = append(s.Query, filterParams(req.Params, "query")...)
	dedupeKV(&s.Query)

	// body
	if req.Body != nil {
		switch req.Body.Type {
		case model.BodyForm:
			s.Form = append(s.Form, req.Body.FormData...)
		case model.BodyMultipart:
			s.Multipart = append(s.Multipart, req.Body.FormData...)
			for _, f := range s.Multipart {
				if strings.HasPrefix(f.Value, "@") {
					s.HasFiles = true
				}
			}
		case model.BodyJSON, model.BodyText, model.BodyXML, model.BodyHTML, "":
			s.Body = req.Body.Data
		}
	}

	// headers + auth
	s.Headers = append(s.Headers, filterParams(req.Headers, "")...)
	if a := req.Auth; a != nil {
		applyAuth(s, a)
	}
	dedupeKV(&s.Headers)

	if settings != nil {
		s.Timeout = settings.Timeout
		s.Follow = settings.FollowRedirects
	}
	s.Vars = collectVars(s.URL, s.Body)
	for _, kv := range s.Headers {
		s.Vars = append(s.Vars, collectVars(kv.Value)...)
	}
	for _, kv := range s.Query {
		s.Vars = append(s.Vars, collectVars(kv.Value)...)
	}
	// re-dedupe/sort after appending
	seen := map[string]bool{}
	var vv []string
	for _, v := range s.Vars {
		if !seen[v] {
			seen[v] = true
			vv = append(vv, v)
		}
	}
	sort.Strings(vv)
	s.Vars = vv
	return s, nil
}

func filterParams(kvs []model.KV, wantType string) []model.KV {
	var out []model.KV
	for _, kv := range kvs {
		if kv.Disabled {
			continue
		}
		if wantType == "" || kv.Type == wantType || kv.Type == "" {
			out = append(out, kv)
		}
	}
	return out
}

func dedupeKV(kvs *[]model.KV) {
	if kvs == nil {
		return
	}
	seen := map[string]bool{}
	var out []model.KV
	for _, kv := range *kvs {
		key := strings.ToLower(kv.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, kv)
	}
	*kvs = out
}

func applyAuth(s *Snippet, a *model.Auth) {
	switch a.Type {
	case model.AuthBearer:
		setHeader(s, "Authorization", "Bearer "+a.Bearer.Token)
	case model.AuthBasic:
		// emitters use native basic-auth when available; fallback: header
		setHeader(s, "Authorization", "Basic <base64("+a.Basic.Username+":"+a.Basic.Password+")>")
		s.AuthNote = "Basic auth for user \"" + a.Basic.Username + "\" — use the target's native basic-auth helper instead of the placeholder header where shown."
	case model.AuthDigest:
		s.AuthNote = "Digest auth for user \"" + a.Digest.Username + "\": the server challenge must be answered at request time; precomputed headers are not possible."
	case model.AuthApiKey:
		if strings.EqualFold(a.ApiKey.In, "query") {
			s.Query = append(s.Query, model.KV{Name: a.ApiKey.Key, Value: a.ApiKey.Value})
		} else {
			setHeader(s, a.ApiKey.Key, a.ApiKey.Value)
		}
	case model.AuthOAuth2:
		s.AuthNote = "OAuth 2 (" + grantLabel(a) + "): obtain an access token from " + a.OAuth2.TokenURL + " first, then set it below."
		setHeader(s, "Authorization", "Bearer {{token}}")
		s.Vars = appendUnique(s.Vars, "token")
	case model.AuthAWSV4:
		s.AuthNote = "AWS SigV4: this request must be signed (x-amz-date, Authorization) with the access/secret keys and region/service; use an AWS SDK or signing helper in the target language."
	case model.AuthWSSE:
		s.AuthNote = "WSSE UsernameToken: PasswordDigest = base64(sha1(nonce + created + password)); generate a fresh nonce per request."
	case model.AuthNTLM:
		s.AuthNote = "NTLM/Negotiate: enable the platform negotiate-auth option or use a library that speaks NTLM."
	}
}

func grantLabel(a *model.Auth) string {
	if a.OAuth2 != nil && a.OAuth2.GrantType != "" {
		return a.OAuth2.GrantType
	}
	return "client_credentials"
}

func setHeader(s *Snippet, name, value string) {
	for i, h := range s.Headers {
		if strings.EqualFold(h.Name, name) {
			s.Headers[i].Value = value
			return
		}
	}
	s.Headers = append(s.Headers, model.KV{Name: name, Value: value})
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// ---------- shared emitter helpers ----------

// headerComment builds the leading comment block listing {{var}} placeholders.
func headerComment(prefix string, s *Snippet, lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		if l != "" {
			b.WriteString(prefix + l + "\n")
		}
	}
	if len(s.Vars) > 0 {
		b.WriteString(prefix + "Replace placeholders: " + strings.Join(mapVars(s.Vars), ", ") + "\n")
	}
	if s.AuthNote != "" {
		b.WriteString(prefix + "Auth: " + s.AuthNote + "\n")
	}
	return b.String()
}

func mapVars(vs []string) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = "{{" + v + "}}"
	}
	return out
}

// effectiveURL returns the full URL with the query string appended.
func effectiveURL(s *Snippet) string {
	if len(s.Query) == 0 {
		return s.URL
	}
	q := make([]string, 0, len(s.Query))
	for _, kv := range s.Query {
		q = append(q, kv.Name+"="+kv.Value)
	}
	sep := "?"
	if strings.Contains(s.URL, "?") {
		sep = "&"
	}
	return s.URL + sep + strings.Join(q, "&")
}
