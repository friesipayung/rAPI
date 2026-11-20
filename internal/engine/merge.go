package engine

// Precedence chain per docs/01-architecture.md §6:
//   variables: collection < folder < environment (extends chain)
//   auth:      request → folder → collection (inherit walks up)
//   headers:   collection defaults < folder defaults < request (by name)

import (
	"strings"

	"github.com/friesipayung/rapi/internal/model"
)

// EnvLookup resolves an environment by name from a collection.
type EnvLookup func(name string) *model.EnvironmentDoc

// MergeVars builds the variable scope: collection, folder, then env chain.
func MergeVars(col *model.CollectionDoc, folder *model.FolderDoc, env *model.EnvironmentDoc, lookup EnvLookup) Vars {
	vars := Vars{}
	add := func(vs []model.Variable) {
		for _, v := range vs {
			if !v.Disabled {
				vars[v.Name] = v.Value
			}
		}
	}
	if col != nil && col.Config.Request != nil {
		add(col.Config.Request.Variables)
	}
	if folder != nil && folder.Request != nil {
		add(folder.Request.Variables)
	}
	// extends chain: base env first, then the env itself
	if env != nil {
		if env.Extends != "" && lookup != nil {
			if base := lookup(env.Extends); base != nil {
				add(base.Variables)
			}
		}
		add(env.Variables)
	}
	return vars
}

// EffectiveAuth resolves the auth chain: request → folder → collection.
// `inherit` (or empty) walks up.
func EffectiveAuth(col *model.CollectionDoc, folder *model.FolderDoc, reqAuth *model.Auth) *model.Auth {
	consider := func(a *model.Auth) *model.Auth {
		if a == nil || a.Type == "" || a.Type == model.AuthInherit {
			return nil
		}
		if a.IsEmpty() {
			return nil
		}
		return a
	}
	if a := consider(reqAuth); a != nil {
		return a
	}
	if folder != nil && folder.Request != nil {
		if a := consider(folder.Request.Auth); a != nil {
			return a
		}
	}
	if col != nil && col.Config.Request != nil {
		if a := consider(col.Config.Request.Auth); a != nil {
			return a
		}
	}
	return nil
}

// MergeHeaders stacks collection < folder < request headers by name.
func MergeHeaders(col *model.CollectionDoc, folder *model.FolderDoc, reqHeaders []model.KV) []model.KV {
	merged := map[string]model.KV{}
	var order []string
	add := func(hs []model.KV) {
		for _, h := range hs {
			if h.Disabled {
				continue
			}
			name := strings.ToLower(h.Name)
			if _, seen := merged[name]; !seen {
				order = append(order, name)
			}
			merged[name] = h
		}
	}
	if col != nil && col.Config.Request != nil {
		add(col.Config.Request.Headers)
	}
	if folder != nil && folder.Request != nil {
		add(folder.Request.Headers)
	}
	add(reqHeaders)
	out := make([]model.KV, 0, len(order))
	for _, name := range order {
		out = append(out, merged[name])
	}
	return out
}

// EffectiveSettings merges settings with request winning; nils fall back to
// collection/folder defaults.
func EffectiveSettings(col *model.CollectionDoc, folder *model.FolderDoc, s *model.Settings) *model.Settings {
	if s == nil {
		if folder != nil && folder.Request != nil && folder.Request.Settings != nil {
			return folder.Request.Settings
		}
		if col != nil && col.Config.Request != nil && col.Config.Request.Settings != nil {
			return col.Config.Request.Settings
		}
		return nil
	}
	return s
}

// Effective interpolates the request for execution and returns the final
// HttpRequest + settings. Does not mutate the input.
func Effective(col *model.CollectionDoc, folder *model.FolderDoc, doc *model.RequestDoc, env *model.EnvironmentDoc, lookup EnvLookup) (*model.HttpRequest, *model.Settings, Vars) {
	vars := MergeVars(col, folder, env, lookup)
	eff, settings := EffectiveWithVars(col, folder, doc, vars)
	return eff, settings, vars
}

// EffectiveWithVars interpolates the request with an explicit scope (used
// after pre-scripts inject runtime variables).
func EffectiveWithVars(col *model.CollectionDoc, folder *model.FolderDoc, doc *model.RequestDoc, vars Vars) (*model.HttpRequest, *model.Settings) {
	in := doc.Http
	effective := &model.HttpRequest{
		Method:  in.Method,
		URL:     Interpolate(in.URL, vars),
		Params:  InterpolateAll(in.Params, vars),
		Headers: InterpolateAll(MergeHeaders(col, folder, in.Headers), vars),
		Body:    nil,
		Auth:    EffectiveAuth(col, folder, in.Auth),
	}
	if in.Body != nil {
		b := *in.Body
		b.Data = Interpolate(b.Data, vars)
		b.FormData = InterpolateAll(b.FormData, vars)
		effective.Body = &b
	}
	if effective.Auth != nil {
		switch {
		case effective.Auth.Basic != nil:
			effective.Auth.Basic.Username = Interpolate(effective.Auth.Basic.Username, vars)
			effective.Auth.Basic.Password = Interpolate(effective.Auth.Basic.Password, vars)
		case effective.Auth.Bearer != nil:
			effective.Auth.Bearer.Token = Interpolate(effective.Auth.Bearer.Token, vars)
		case effective.Auth.ApiKey != nil:
			effective.Auth.ApiKey.Key = Interpolate(effective.Auth.ApiKey.Key, vars)
			effective.Auth.ApiKey.Value = Interpolate(effective.Auth.ApiKey.Value, vars)
		case effective.Auth.Digest != nil:
			effective.Auth.Digest.Username = Interpolate(effective.Auth.Digest.Username, vars)
			effective.Auth.Digest.Password = Interpolate(effective.Auth.Digest.Password, vars)
		case effective.Auth.OAuth2 != nil:
			o := *effective.Auth.OAuth2
			o.TokenURL = Interpolate(o.TokenURL, vars)
			o.ClientID = Interpolate(o.ClientID, vars)
			o.ClientSecret = Interpolate(o.ClientSecret, vars)
			o.Username = Interpolate(o.Username, vars)
			o.Password = Interpolate(o.Password, vars)
			o.Scopes = Interpolate(o.Scopes, vars)
			effective.Auth.OAuth2 = &o
		case effective.Auth.AWSV4 != nil:
			w := *effective.Auth.AWSV4
			w.AccessKey = Interpolate(w.AccessKey, vars)
			w.SecretKey = Interpolate(w.SecretKey, vars)
			w.SessionToken = Interpolate(w.SessionToken, vars)
			w.Region = Interpolate(w.Region, vars)
			w.Service = Interpolate(w.Service, vars)
			effective.Auth.AWSV4 = &w
		case effective.Auth.WSSE != nil:
			s := *effective.Auth.WSSE
			s.Username = Interpolate(s.Username, vars)
			s.Password = Interpolate(s.Password, vars)
			effective.Auth.WSSE = &s
		}
	}
	return effective, EffectiveSettings(col, folder, doc.Settings)
}
