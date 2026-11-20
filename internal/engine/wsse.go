package engine

// WSSE UsernameToken (X-WSSE header), password digest profile:
//   digest = base64(sha1(nonce + created + password))

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

// SignWSSE sets the X-WSSE header on r.
func SignWSSE(r *http.Request, w *model.WSSEAuth) error {
	if w == nil || w.Username == "" {
		return fmt.Errorf("wsse: username is required")
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)
	created := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	h := sha1.Sum(append(append(nonce, []byte(created)...), []byte(w.Password)...))
	digest := base64.StdEncoding.EncodeToString(h[:])
	r.Header.Set("X-WSSE", fmt.Sprintf(
		`UsernameToken Username="%s", PasswordDigest="%s", Nonce="%s", Created="%s"`,
		w.Username, digest, nonceB64, created))
	r.Header.Set("Authorization", "WSSE profile=\"UsernameToken\"")
	return nil
}
