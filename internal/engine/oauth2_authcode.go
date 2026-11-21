package engine

// OAuth2 authorization_code grant with optional PKCE (RFC 7636).
// The flow opens the system browser at the authorization endpoint, captures
// the redirect on a loopback listener and exchanges the code. Access and
// refresh tokens are cached in memory only — they never touch the vault.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

const (
	authCodeTimeout = 2 * time.Minute // user has 2 min to finish the browser dance
	stateLength     = 16
	verifierLength  = 64 // bytes → ~86 base64url chars
)

// fetchAuthorizationCode runs the interactive flow. `form` is the partially
// built token-request form (grant_type + scope already set).
func fetchAuthorizationCode(ctx context.Context, o *model.OAuth2Auth, form url.Values) (string, error) {
	if strings.TrimSpace(o.AuthURL) == "" {
		return "", fmt.Errorf("oauth2: authorization endpoint (authUrl) is empty")
	}
	// 1. try the in-memory refresh token
	key := o.TokenURL + "|" + o.ClientID + "|" + o.Scopes + "|" + o.Username
	if rt := getRefreshToken(key); rt != "" {
		if tok, err := refreshAccessToken(ctx, o, form, rt); err == nil {
			return tok, nil
		}
		// refresh failed — fall through to a fresh interactive round
	}

	// 2. loopback listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("oauth2: loopback listener: %w", err)
	}
	defer ln.Close()
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)

	verifier, challenge := "", ""
	if o.UsePKCE {
		verifier, challenge = pkcePair()
	}

	state := randomB64(stateLength)
	authURL, err := buildAuthURL(o.AuthURL, o.ClientID, redirectURI, o.Scopes, state, challenge)
	if err != nil {
		return "", err
	}

	codeCh := make(chan authCodeResult, 1)
	srv := &http.Server{Handler: loopbackHandler(state, codeCh)}
	go func() {
		_ = srv.Serve(ln)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := openBrowser(authURL); err != nil {
		return "", fmt.Errorf("oauth2: cannot open browser: %w (authorization URL: %s)", err, authURL)
	}

	var res authCodeResult
	select {
	case res = <-codeCh:
	case <-time.After(authCodeTimeout):
		return "", fmt.Errorf("oauth2: timed out waiting for the authorization redirect")
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if res.err != nil {
		return "", res.err
	}

	// 3. exchange the code
	tok, refresh, err := exchangeAuthorizationCode(ctx, o, form, res.code, verifier, redirectURI)
	if err != nil {
		return "", err
	}
	if refresh != "" {
		setRefreshToken(key, refresh)
	}
	return tok, nil
}

type authCodeResult struct {
	code string
	err  error
}

// loopbackHandler captures ?code=...&state=... from the redirect.
func loopbackHandler(state string, ch chan<- authCodeResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorization failed, see the app", http.StatusBadRequest)
			ch <- authCodeResult{err: fmt.Errorf("oauth2: authorization error %q: %s", e, q.Get("error_description"))}
			return
		}
		if q.Get("state") != state {
			http.Error(w, "state mismatch, see the app", http.StatusBadRequest)
			ch <- authCodeResult{err: errors.New("oauth2: state mismatch (possible CSRF)")}
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "Authorization complete. You can close this tab and return to rAPI.")
		ch <- authCodeResult{code: q.Get("code")}
	})
}

func buildAuthURL(authURL, clientID, redirectURI, scopes, state, challenge string) (string, error) {
	u, err := url.Parse(authURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("oauth2: invalid authorization endpoint %q", authURL)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	if scopes != "" {
		q.Set("scope", scopes)
	}
	if challenge != "" {
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// exchangeAuthorizationCode POSTs the code to the token endpoint.
func exchangeAuthorizationCode(ctx context.Context, o *model.OAuth2Auth, form url.Values, code, verifier, redirectURI string) (access, refresh string, err error) {
	f := cloneValues(form)
	f.Set("code", code)
	f.Set("redirect_uri", redirectURI)
	if o.ClientID != "" {
		f.Set("client_id", o.ClientID)
	}
	if verifier != "" {
		f.Set("code_verifier", verifier)
	}
	body, err := postTokenURL(ctx, o, f, o.ClientID != "" && o.ClientSecret != "")
	if err != nil {
		return "", "", err
	}
	tok, err := parseTokenResponse(body)
	if err != nil {
		return "", "", err
	}
	return tok.accessToken, tok.refreshToken, nil
}

// refreshAccessToken uses a cached refresh token.
func refreshAccessToken(ctx context.Context, o *model.OAuth2Auth, form url.Values, refreshToken string) (string, error) {
	f := cloneValues(form)
	f.Set("grant_type", "refresh_token")
	f.Set("refresh_token", refreshToken)
	if o.ClientID != "" {
		f.Set("client_id", o.ClientID)
	}
	body, err := postTokenURL(ctx, o, f, o.ClientID != "" && o.ClientSecret != "")
	if err != nil {
		return "", err
	}
	tok, err := parseTokenResponse(body)
	if err != nil {
		return "", err
	}
	if tok.refreshToken != "" {
		key := o.TokenURL + "|" + o.ClientID + "|" + o.Scopes + "|" + o.Username
		setRefreshToken(key, tok.refreshToken)
	}
	return tok.accessToken, nil
}

// ---------- shared token plumbing ----------

type parsedToken struct {
	accessToken  string
	refreshToken string
	expiresIn    int
}

func parseTokenResponse(body []byte) (parsedToken, error) {
	var tok struct {
		AccessToken string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := jsonUnmarshalStrict(body, &tok); err != nil || tok.AccessToken == "" {
		return parsedToken{}, fmt.Errorf("oauth2: no access_token in token response")
	}
	return parsedToken{accessToken: tok.AccessToken, refreshToken: tok.RefreshToken, expiresIn: tok.ExpiresIn}, nil
}

func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// ---------- PKCE (RFC 7636) ----------

// pkcePair returns (verifier, S256 challenge).
func pkcePair() (verifier, challenge string) {
	verifier = randomB64(verifierLength)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

func randomB64(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---------- refresh token store (in-memory only) ----------

var refreshMu = make(map[string]string) // guarded by tokMu from oauth2.go

func getRefreshToken(key string) string {
	tokMu.Lock()
	defer tokMu.Unlock()
	return refreshMu[key]
}

func setRefreshToken(key, token string) {
	tokMu.Lock()
	defer tokMu.Unlock()
	refreshMu[key] = token
}

// ---------- browser ----------

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
