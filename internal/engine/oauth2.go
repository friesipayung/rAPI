package engine

// OAuth2 (client_credentials & password grants) with an in-memory token
// cache. Tokens never touch the vault — they live for the app session only.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

const oauthTimeout = 15 * time.Second

type cachedToken struct {
	token     string
	expiresAt time.Time
}

var (
	tokMu    sync.Mutex
	tokCache = map[string]cachedToken{}
)

// FetchOAuth2Token obtains an access token, using the cache when the current
// token is still valid (60s safety margin).
func FetchOAuth2Token(ctx context.Context, o *model.OAuth2Auth) (string, error) {
	if o == nil || strings.TrimSpace(o.TokenURL) == "" {
		return "", fmt.Errorf("oauth2: token URL is empty")
	}
	key := o.TokenURL + "|" + o.ClientID + "|" + o.Scopes + "|" + o.Username
	tokMu.Lock()
	if c, ok := tokCache[key]; ok && time.Now().Before(c.expiresAt) {
		tokMu.Unlock()
		return c.token, nil
	}
	tokMu.Unlock()

	grant := strings.ToLower(strings.TrimSpace(o.GrantType))
	if grant == "" {
		grant = "client_credentials"
	}
	form := url.Values{"grant_type": {grant}}
	if o.Scopes != "" {
		form.Set("scope", o.Scopes)
	}
	switch grant {
	case "client_credentials":
		if o.ClientID != "" || o.ClientSecret != "" {
			form.Set("client_id", o.ClientID)
			form.Set("client_secret", o.ClientSecret)
		}
	case "password":
		form.Set("client_id", o.ClientID)
		if o.ClientSecret != "" {
			form.Set("client_secret", o.ClientSecret)
		}
		form.Set("username", o.Username)
		form.Set("password", o.Password)
	default:
		return "", fmt.Errorf("oauth2: unsupported grant type %q (use client_credentials or password)", grant)
	}

	ctx, cancel := context.WithTimeout(ctx, oauthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if grant == "client_credentials" && o.ClientID != "" && o.ClientSecret != "" {
		req.SetBasicAuth(o.ClientID, o.ClientSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth2 token request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth2 token request failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("oauth2: no access_token in response")
	}
	exp := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if tok.ExpiresIn <= 0 {
		exp = time.Now().Add(5 * time.Minute)
	}
	tokMu.Lock()
	tokCache[key] = cachedToken{token: tok.AccessToken, expiresAt: exp.Add(-60 * time.Second)}
	tokMu.Unlock()
	return tok.AccessToken, nil
}

// InvalidateOAuth2Token drops the cached token for the given config.
func InvalidateOAuth2Token(o *model.OAuth2Auth) {
	if o == nil {
		return
	}
	key := o.TokenURL + "|" + o.ClientID + "|" + o.Scopes + "|" + o.Username
	tokMu.Lock()
	delete(tokCache, key)
	tokMu.Unlock()
}
