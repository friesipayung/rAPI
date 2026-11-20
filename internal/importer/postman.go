package importer

// Postman Collection v2.1 → OpenCollection.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
)

func parseJSON(b []byte, out any) error { return json.Unmarshal(b, out) }

type postmanCollection struct {
	Info struct {
		Name   string `json:"name"`
		Schema string `json:"schema"`
	} `json:"info"`
	Item []postmanItem `json:"item"`
	Auth any           `json:"auth"`
}

type postmanItem struct {
	Name     string        `json:"name"`
	Item     []postmanItem `json:"item"` // folders nest
	Request  *postmanReq   `json:"request,omitempty"`
	Response []postmanResp `json:"response,omitempty"`
}

type postmanReq struct {
	Method      string          `json:"method"`
	URL         json.RawMessage `json:"url"` // string or object
	Header      []postmanKV     `json:"header,omitempty"`
	Body        *postmanBody    `json:"body,omitempty"`
	Auth        *postmanAuth    `json:"auth,omitempty"`
	Description any             `json:"description,omitempty"` // string or {content}
}

type postmanKV struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Disabled   bool   `json:"disabled,omitempty"`
	Type       string `json:"type,omitempty"`
	Secret     bool   `json:"secret,omitempty"`
	Desc       any    `json:"description,omitempty"`
	Src        string `json:"src,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

type postmanBody struct {
	Mode       string      `json:"mode,omitempty"` // raw|urlencoded|formdata|file
	Raw        string      `json:"raw,omitempty"`
	URLEncoded []postmanKV `json:"urlencoded,omitempty"`
	FormData   []postmanKV `json:"formdata,omitempty"`
	Options    struct {
		Raw struct {
			Language string `json:"language,omitempty"` // json|text|html|xml
		} `json:"raw,omitempty"`
	} `json:"options,omitempty"`
}

type postmanAuth struct {
	Type   string          `json:"type,omitempty"` // basic|bearer|apikey|digest|...
	Basic  []postmanKV     `json:"basic,omitempty"`
	Bearer []postmanKV     `json:"bearer,omitempty"`
	ApiKey []postmanKV     `json:"apikey,omitempty"`
	Digest []postmanKV     `json:"digest,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

func (a *postmanAuth) param(kind, key string) string {
	var kvs []postmanKV
	switch kind {
	case "basic":
		kvs = a.Basic
	case "bearer":
		kvs = a.Bearer
	case "apikey":
		kvs = a.ApiKey
	case "digest":
		kvs = a.Digest
	}
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

type postmanResp struct {
	Name       string      `json:"name"`
	Status     string      `json:"status"`
	Code       int         `json:"code"`
	Header     []postmanKV `json:"header,omitempty"`
	Body       string      `json:"body,omitempty"`
	OrigReq    any         `json:"originalRequest,omitempty"`
}

func parsePostman(path string) (*tree, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var col postmanCollection
	if err := parseJSON(b, &col); err != nil {
		return nil, fmt.Errorf("invalid Postman JSON: %w", err)
	}
	if col.Info.Name == "" {
		return nil, fmt.Errorf("missing info.name (is this a Postman collection?)")
	}
	root := &tree{name: col.Info.Name}
	walkPostman(root, col.Item)
	return root, nil
}

func walkPostman(parent *tree, items []postmanItem) {
	for _, it := range items {
		if len(it.Item) > 0 || it.Request == nil {
			ch := &tree{name: it.Name}
			parent.children = append(parent.children, ch)
			walkPostman(ch, it.Item)
			continue
		}
		parent.requests = append(parent.requests, postmanToRequest(it))
	}
}

func postmanToRequest(it postmanItem) *model.RequestDoc {
	rd := &model.RequestDoc{}
	rd.Info.Name = it.Name
	rd.Info.Type = "http"
	req := it.Request
	h := &rd.Http
	h.Method = strings.ToUpper(firstNonEmptyStr(req.Method, "GET"))

	// URL: string or {raw, query, ...}
	var rawURL string
	var query []postmanKV
	if len(req.URL) > 0 {
		var s string
		if parseJSON(req.URL, &s) == nil {
			rawURL = s
		} else {
			var u struct {
				Raw   string      `json:"raw"`
				Query []postmanKV `json:"query"`
			}
			if parseJSON(req.URL, &u) == nil {
				rawURL = u.Raw
				query = u.Query
			}
		}
	}
	h.URL = strings.TrimSuffix(rawURL, "?")
	for _, q := range query {
		h.Params = append(h.Params, model.KV{Name: q.Key, Value: q.Value, Disabled: q.Disabled})
	}
	for _, hd := range req.Header {
		h.Headers = append(h.Headers, model.KV{Name: hd.Key, Value: hd.Value, Disabled: hd.Disabled})
	}

	// body
	if req.Body != nil {
		switch req.Body.Mode {
		case "urlencoded":
			h.Body = &model.Body{Type: model.BodyForm}
			for _, f := range req.Body.URLEncoded {
				h.Body.FormData = append(h.Body.FormData, model.KV{Name: f.Key, Value: f.Value, Disabled: f.Disabled})
			}
		case "formdata":
			h.Body = &model.Body{Type: model.BodyMultipart}
			for _, f := range req.Body.FormData {
				h.Body.FormData = append(h.Body.FormData, model.KV{Name: f.Key, Value: firstNonEmptyStr(f.Value, f.Src), Disabled: f.Disabled})
			}
		case "raw", "":
			h.Body = &model.Body{Type: "text"}
			h.Body.Data = req.Body.Raw
			switch req.Body.Options.Raw.Language {
			case "json":
				h.Body.Type = model.BodyJSON
			case "xml":
				h.Body.Type = model.BodyXML
			case "html":
				h.Body.Type = model.BodyHTML
			}
		}
	}

	// auth
	h.Auth = postmanToAuth(req.Auth)

	// description → docs
	rd.Docs = descText(req.Description)

	// saved responses → examples
	var examples []model.HttpRequestExample
	for _, r := range it.Response {
		ex := model.HttpRequestExample{
			Name: firstNonEmptyStr(r.Name, fmt.Sprintf("Example %d (%d)", len(examples)+1, r.Code)),
		}
		ex.Response = &model.ExampleResponse{
			Status:     r.Code,
			StatusText: r.Status,
			Headers:    kvToHeaders(r.Header),
		}
		ex.Response.Body = guessBodyType(r.Body)
		examples = append(examples, ex)
	}
	_ = rd.SetExamples(examples)
	return rd
}

func postmanToAuth(a *postmanAuth) *model.Auth {
	if a == nil {
		return nil
	}
	switch a.Type {
	case "basic":
		return &model.Auth{Type: model.AuthBasic, Basic: &model.BasicAuth{
			Username: a.param("basic", "username"),
			Password: a.param("basic", "password"),
		}}
	case "bearer":
		return &model.Auth{Type: model.AuthBearer, Bearer: &model.BearerAuth{Token: a.param("bearer", "token")}}
	case "apikey":
		in := firstNonEmptyStr(a.param("apikey", "in"), "header")
		return &model.Auth{Type: model.AuthApiKey, ApiKey: &model.ApiKeyAuth{
			Key:   a.param("apikey", "key"),
			Value: a.param("apikey", "value"),
			In:    in,
		}}
	case "digest":
		return &model.Auth{Type: model.AuthDigest, Digest: &model.DigestAuth{
			Username: a.param("digest", "username"),
			Password: a.param("digest", "password"),
		}}
	}
	return nil
}

func descText(v any) string {
	switch d := v.(type) {
	case nil:
		return ""
	case string:
		return d
	case map[string]any:
		if c, ok := d["content"].(string); ok {
			return c
		}
	}
	return ""
}

func kvToHeaders(in []postmanKV) []model.KV {
	var out []model.KV
	for _, h := range in {
		out = append(out, model.KV{Name: h.Key, Value: h.Value})
	}
	return out
}

func guessBodyType(body string) *model.ResponseBody {
	if body == "" {
		return nil
	}
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return &model.ResponseBody{Type: "json", Data: body}
	}
	if strings.HasPrefix(trimmed, "<") {
		return &model.ResponseBody{Type: "xml", Data: body}
	}
	return &model.ResponseBody{Type: "text", Data: body}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
