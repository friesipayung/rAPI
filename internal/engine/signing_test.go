package engine

// SigV4 + WSSE signing tests, incl. the AWS docs example vector.

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

// TestSignAWSV4KnownVector replicates the AWS "ListUsers" SigV4 example:
// https://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html
func TestSignAWSV4KnownVector(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://iam.amazonaws.com/?Action=ListUsers&Version=2010-05-08", nil)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	a := &model.AWSV4Auth{
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "us-east-1",
		Service:   "iam",
	}
	when, err := time.Parse(amzDateFormat, "20150830T123600Z")
	if err != nil {
		t.Fatal(err)
	}
	if err := SignAWSV4(r, a, nil, when); err != nil {
		t.Fatalf("sign: %v", err)
	}
	auth := r.Header.Get("Authorization")
	const wantSig = "Signature=5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7"
	if !strings.Contains(auth, wantSig) {
		t.Fatalf("signature mismatch:\n got: %s\nwant: %s", auth, wantSig)
	}
	if !strings.Contains(auth, "Credential=AKIDEXAMPLE/20150830/us-east-1/iam/aws4_request") {
		t.Fatalf("credential scope missing: %s", auth)
	}
	if r.Header.Get("x-amz-date") != "20150830T123600Z" {
		t.Fatalf("x-amz-date = %q", r.Header.Get("x-amz-date"))
	}
}

func TestSignAWSV4MissingFields(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://s3.amazonaws.com/bucket", nil)
	if err := SignAWSV4(r, &model.AWSV4Auth{AccessKey: "AKID"}, nil, time.Now()); err == nil {
		t.Fatal("expected error for missing region/service")
	}
}

func TestSignWSSEHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://x.test/api", nil)
	if err := SignWSSE(r, &model.WSSEAuth{Username: "bob", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	h := r.Header.Get("X-WSSE")
	if !strings.HasPrefix(h, `UsernameToken Username="bob"`) {
		t.Fatalf("X-WSSE = %q", h)
	}
	for _, part := range []string{"PasswordDigest=", "Nonce=", "Created="} {
		if !strings.Contains(h, part) {
			t.Fatalf("X-WSSE missing %s: %q", part, h)
		}
	}
	if r.Header.Get("Authorization") == "" {
		t.Fatal("missing WSSE Authorization profile header")
	}
	// two calls must produce different nonces
	r2, _ := http.NewRequest("GET", "https://x.test/api", nil)
	_ = SignWSSE(r2, &model.WSSEAuth{Username: "bob", Password: "secret"})
	if r.Header.Get("X-WSSE") == r2.Header.Get("X-WSSE") {
		t.Fatal("nonce reuse across calls")
	}
}

func TestSignWSSEMissingUser(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://x.test", nil)
	if err := SignWSSE(r, &model.WSSEAuth{Username: "", Password: "x"}); err == nil {
		t.Fatal("expected error for empty username")
	}
}
