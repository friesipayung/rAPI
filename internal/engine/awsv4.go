package engine

// AWS Signature Version 4 signing (awsv4 auth). Pure stdlib; verified against
// the AWS docs "ListUsers" example vector in runtime_test.go.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

const (
	sigAlgorithm  = "AWS4-HMAC-SHA256"
	amzDateFormat = "20060102T150405Z"
	amzDateOnly   = "20060102"
)

// SignAWSV4 signs r in place with SigV4. `now` is injectable for tests.
func SignAWSV4(r *http.Request, a *model.AWSV4Auth, body []byte, now time.Time) error {
	if a == nil || a.AccessKey == "" || a.Region == "" || a.Service == "" {
		return fmt.Errorf("awsv4: access key, region and service are required")
	}
	amzDate := now.UTC().Format(amzDateFormat)
	dateStamp := now.UTC().Format(amzDateOnly)

	// payload hash
	payloadHash := sha256.Sum256(body)
	payloadHex := hex.EncodeToString(payloadHash[:])

	// canonical URI (path, URI-encoded per segment but slashes kept)
	canonicalURI := r.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// canonical query (sorted by key, then value)
	q := r.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var qb strings.Builder
	for _, k := range keys {
		vals := q[k]
		sort.Strings(vals)
		for _, v := range vals {
			qb.WriteString(awsURIEncode(k, true))
			qb.WriteByte('=')
			qb.WriteString(awsURIEncode(v, true))
			qb.WriteByte('&')
		}
	}
	canonicalQuery := strings.TrimSuffix(qb.String(), "&")

	// canonical headers: host, x-amz-date, x-amz-security-token, plus
	// existing headers that AWS clients conventionally sign (content-type).
	host := r.URL.Host
	headers := map[string]string{
		"host":      host,
		"x-amz-date": amzDate,
	}
	if a.SessionToken != "" {
		headers["x-amz-security-token"] = a.SessionToken
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		headers["content-type"] = ct
	}
	signed := make([]string, 0, len(headers))
	for k := range headers {
		signed = append(signed, k)
	}
	sort.Strings(signed)
	var hb strings.Builder
	for _, k := range signed {
		hb.WriteString(k)
		hb.WriteByte(':')
		hb.WriteString(strings.TrimSpace(headers[k]))
		hb.WriteByte('\n')
	}
	canonicalHeaders := hb.String()
	signedHeaders := strings.Join(signed, ";")

	canonicalRequest := strings.Join([]string{
		r.Method, canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, payloadHex,
	}, "\n")

	crHash := sha256.Sum256([]byte(canonicalRequest))
	scope := dateStamp + "/" + a.Region + "/" + a.Service + "/aws4_request"
	stringToSign := strings.Join([]string{
		sigAlgorithm, amzDate, scope, hex.EncodeToString(crHash[:]),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+a.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, a.Region)
	kService := hmacSHA256(kRegion, a.Service)
	kSigning := hmacSHA256(kService, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	r.Header.Set("x-amz-date", amzDate)
	if a.SessionToken != "" {
		r.Header.Set("x-amz-security-token", a.SessionToken)
	}
	r.Header.Set("Authorization", fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		sigAlgorithm, a.AccessKey, scope, signedHeaders, sig))
	return nil
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// awsURIEncode implements the AWS URI encoding (RFC 3986, unreserved
// characters kept; space becomes %20).
func awsURIEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/':
			if encodeSlash {
				b.WriteString("%2F")
			} else {
				b.WriteByte(c)
			}
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
