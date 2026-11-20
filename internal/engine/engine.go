package engine

// HTTP execution engine (phase 2): {{var}} interpolation, request building,
// auth (basic/bearer/apikey/digest), settings (timeout, redirects, encodeUrl).

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

const (
	defaultTimeoutSecs = 30
	maxBodyBytes       = 10 << 20 // 10 MB response cap
)

var varRe = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*\}\}`)

// Vars is the merged variable scope for one execution.
type Vars map[string]string

// Interpolate replaces {{name}} with scope values; unknown vars are left
// verbatim so the user can see they are unresolved.
func Interpolate(s string, vars Vars) string {
	if s == "" || len(vars) == 0 {
		return s
	}
	return varRe.ReplaceAllStringFunc(s, func(m string) string {
		name := varRe.FindStringSubmatch(m)[1]
		if v, ok := vars[name]; ok {
			return v
		}
		return m
	})
}

// InterpolateAll replaces vars in every string value of a slice.
func InterpolateAll(in []model.KV, vars Vars) []model.KV {
	out := make([]model.KV, len(in))
	for i, kv := range in {
		kv.Value = Interpolate(kv.Value, vars)
		out[i] = kv
	}
	return out
}

// EnabledKV filters out disabled entries.
func EnabledKV(in []model.KV) []model.KV {
	out := make([]model.KV, 0, len(in))
	for _, kv := range in {
		if !kv.Disabled {
			out = append(out, kv)
		}
	}
	return out
}

// ---------- execution ----------

// Executor runs interpolated requests. Defaults are pre-merged by the caller
// (RunService): it passes the effective request and auth chain.
type Executor struct {
	// InsecureSkipVerify stays false; proxy/TLS options arrive in phase 4.
}

// Run executes the given (already interpolated) request against the real
// network. Network/protocol errors come back in RunResult.Error.
func (e *Executor) Run(req *model.HttpRequest, settings *model.Settings, envUsed string) *model.RunResult {
	res := &model.RunResult{Headers: []model.KV{}, EnvUsed: envUsed}
	if req.URL == "" {
		res.Error = "URL is empty"
		return res
	}

	timeout := defaultTimeoutSecs
	if settings != nil && settings.Timeout > 0 {
		timeout = settings.Timeout
	}
	follow := true
	maxRedirects := 10
	if settings != nil {
		if settings.FollowRedirects != nil {
			follow = *settings.FollowRedirects
		}
		if settings.MaxRedirects > 0 {
			maxRedirects = settings.MaxRedirects
		}
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if !follow {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	httpReq, bodyBytes, ctype, err := e.buildRequest(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		// digest auth: retry once on 401 with WWW-Authenticate: Digest
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()

	// digest challenge-response
	if resp.StatusCode == http.StatusUnauthorized && digestChallenge(resp.Header.Get("WWW-Authenticate")) != nil && req.Auth != nil && req.Auth.Type == model.AuthDigest {
		resp.Body.Close()
		ch := digestChallenge(resp.Header.Get("WWW-Authenticate"))
		authHeader := digestResponse(ch, req.Auth.Digest, httpReq.Method, requestURI(httpReq.URL), bodyBytes)
		httpReq2, _, _, err2 := e.buildRequest(req)
		if err2 == nil {
			httpReq2.Header.Set("Authorization", authHeader)
			start = time.Now()
			resp2, err2 := client.Do(httpReq2)
			if err2 != nil {
				res.Error = err2.Error()
				return res
			}
			defer resp2.Body.Close()
			e.fillResult(res, resp2, start)
			e.snapshot(res, req)
			return res
		}
	}

	e.fillResult(res, resp, start)
	e.snapshot(res, req)
	_ = ctype
	return res
}

func (e *Executor) fillResult(res *model.RunResult, resp *http.Response, start time.Time) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		res.Error = "reading response: " + err.Error()
	}
	res.Status = resp.StatusCode
	res.StatusText = resp.Status
	res.TimeMs = time.Since(start).Milliseconds()
	res.SizeBytes = len(body)
	for k, vs := range resp.Header {
		res.Headers = append(res.Headers, model.KV{Name: k, Value: strings.Join(vs, ", ")})
	}
	res.Body = classifyBody(resp.Header.Get("Content-Type"), body)
}

func (e *Executor) snapshot(res *model.RunResult, req *model.HttpRequest) {
	res.Request = &model.RequestSnapshot{
		Method:  req.Method,
		URL:     req.URL,
		Params:  req.Params,
		Headers: req.Headers,
		Body:    req.Body,
	}
}

// classifyBody maps a content type + bytes to a ResponseBody.
func classifyBody(contentType string, body []byte) *model.ResponseBody {
	rb := &model.ResponseBody{Data: string(body)}
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		rb.Type = "json"
		var buf bytes.Buffer
		if json.Indent(&buf, body, "", "  ") == nil {
			rb.Data = buf.String()
		}
	case strings.Contains(ct, "xml"):
		rb.Type = "xml"
	case strings.Contains(ct, "html"):
		rb.Type = "html"
	case strings.HasPrefix(ct, "text/"), ct == "":
		rb.Type = "text"
	default:
		rb.Type = "binary"
		rb.Data = base64.StdEncoding.EncodeToString(body)
	}
	return rb
}

// buildRequest assembles the *http.Request from the model (auth headers,
// body encoding, query params).
func (e *Executor) buildRequest(req *model.HttpRequest) (*http.Request, []byte, string, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	rawURL := req.URL
	// query params of type=query (or untyped) appended to the URL
	params := EnabledKV(req.Params)
	var qp url.Values
	for _, p := range params {
		if p.Type == "" || p.Type == "query" {
			if qp == nil {
				qp = url.Values{}
			}
			qp.Add(p.Name, p.Value)
		}
	}
	if len(qp) > 0 {
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		rawURL = rawURL + sep + qp.Encode()
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" {
		return nil, nil, "", fmt.Errorf("URL missing scheme (use http:// or https://)")
	}

	var bodyReader io.Reader
	var bodyBytes []byte
	var ctype string

	if req.Body != nil && method != http.MethodGet && method != http.MethodHead {
		switch req.Body.Type {
		case model.BodyForm:
			form := url.Values{}
			for _, f := range EnabledKV(req.Body.FormData) {
				form.Add(f.Name, f.Value)
			}
			bodyBytes = []byte(form.Encode())
			ctype = "application/x-www-form-urlencoded"
		case model.BodyMultipart:
			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)
			for _, f := range EnabledKV(req.Body.FormData) {
				if err := w.WriteField(f.Name, f.Value); err != nil {
					return nil, nil, "", err
				}
			}
			if err := w.Close(); err != nil {
				return nil, nil, "", err
			}
			bodyBytes = buf.Bytes()
			ctype = w.FormDataContentType()
		case "", model.BodyJSON, model.BodyText, model.BodyXML, model.BodyHTML:
			bodyBytes = []byte(req.Body.Data)
			switch req.Body.Type {
			case model.BodyJSON:
				ctype = "application/json"
			case model.BodyXML:
				ctype = "application/xml"
			case model.BodyHTML:
				ctype = "text/html"
			default:
				ctype = "text/plain"
			}
		}
	}
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		return nil, nil, "", err
	}

	// headers (request-level)
	for _, h := range EnabledKV(req.Headers) {
		if h.Type == "" || h.Type == "header" {
			httpReq.Header.Set(h.Name, h.Value)
		}
	}
	if ctype != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", ctype)
	}
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "rAPI/1.0")
	}

	// auth
	if err := e.applyAuth(httpReq, u, req.Auth, bodyBytes); err != nil {
		return nil, nil, "", err
	}
	return httpReq, bodyBytes, ctype, nil
}

func (e *Executor) applyAuth(r *http.Request, u *url.URL, auth *model.Auth, _ []byte) error {
	if auth == nil || auth.IsEmpty() {
		return nil
	}
	switch auth.Type {
	case model.AuthBasic:
		r.SetBasicAuth(auth.Basic.Username, auth.Basic.Password)
	case model.AuthBearer:
		r.Header.Set("Authorization", "Bearer "+auth.Bearer.Token)
	case model.AuthApiKey:
		switch strings.ToLower(firstNonEmpty(auth.ApiKey.In, "header")) {
		case "query":
			q := u.Query()
			q.Add(auth.ApiKey.Key, auth.ApiKey.Value)
			r.URL.RawQuery = q.Encode()
		default:
			r.Header.Set(auth.ApiKey.Key, auth.ApiKey.Value)
		}
	case model.AuthDigest:
		// handled after the 401 challenge (see Run)
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func requestURI(u *url.URL) string {
	uri := u.RequestURI()
	if uri == "" {
		uri = "/"
	}
	return uri
}

// ---------- digest auth (RFC 2617/7616, qop=auth) ----------

type digestParams map[string]string

func digestChallenge(header string) digestParams {
	if !strings.HasPrefix(strings.ToLower(header), "digest ") {
		return nil
	}
	p := digestParams{}
	for _, part := range strings.Split(header[7:], ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		p[strings.ToLower(kv[0])] = strings.Trim(kv[1], `"`)
	}
	if p["nonce"] == "" {
		return nil
	}
	return p
}

func digestHash(alg, s string) string {
	switch strings.ToUpper(strings.TrimSpace(alg)) {
	case "SHA-256", "SHA-256-SESS":
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	default: // MD5, MD5-SESS, ""
		sum := md5.Sum([]byte(s))
		return hex.EncodeToString(sum[:])
	}
}

func digestResponse(p digestParams, d *model.DigestAuth, method, uri string, body []byte) string {
	alg := firstNonEmpty(p["algorithm"], "MD5")
	ha1 := digestHash(alg, d.Username+":"+p["realm"]+":"+d.Password)
	if strings.HasSuffix(strings.ToUpper(alg), "-SESS") {
		cnonce := randomHex(8)
		ha1 = digestHash(alg, ha1+":"+p["nonce"]+":"+cnonce)
	}
	ha2 := digestHash(alg, method+":"+uri)
	if q := p["qop"]; strings.Contains(q, "auth-int") && q != "" {
		ha2 = digestHash(alg, method+":"+uri+":"+digestHash("", string(body)))
	}
	nc := "00000001"
	cnonce := randomHex(8)
	qop := p["qop"]
	respStr := ""
	if qop == "" {
		respStr = digestHash(alg, ha1+":"+p["nonce"]+":"+ha2)
	} else {
		respStr = digestHash(alg, ha1+":"+p["nonce"]+":"+nc+":"+cnonce+":"+qop+":"+ha2)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		d.Username, p["realm"], p["nonce"], uri, respStr)
	if p["opaque"] != "" {
		fmt.Fprintf(&b, `, opaque="%s"`, p["opaque"])
	}
	if p["algorithm"] != "" {
		fmt.Fprintf(&b, `, algorithm=%s`, p["algorithm"])
	}
	if qop != "" {
		fmt.Fprintf(&b, `, qop=%s, nc=%s, cnonce="%s"`, qop, nc, cnonce)
	}
	return b.String()
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
