package runner

// JUnit XML rendering of a Report (--report junit), for CI integration.
// Built with encoding/xml struct marshaling so all attribute/text values are
// escaped by the encoder — never by string concatenation.

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// junitTime renders milliseconds as JUnit's decimal-seconds time attribute.
func junitTime(ms int64) string {
	return strconv.FormatFloat(float64(ms)/1000.0, 'f', 3, 64)
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type junitSkipped struct {
	Message string `xml:"message,attr,omitempty"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
}

type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

// MarshalJunit renders the report as a JUnit XML document (with the standard
// XML declaration). classname groups steps by collection (first id segment);
// name is the step's display name, falling back to its request id. A failed
// step carries a <failure> child whose message states the primary reason;
// a step skipped by StopOnFailure carries <skipped/>.
func (r *Report) MarshalJunit() []byte {
	suite := junitSuite{Name: "rapi", Time: junitTime(r.TotalMs)}
	if r == nil {
		b, _ := xml.MarshalIndent(&suite, "", "  ")
		return append([]byte(xml.Header), b...)
	}
	suite.Tests = len(r.Steps)
	suite.Failures = r.FailCount
	for _, sr := range r.Steps {
		c := junitCase{
			Name:      sr.Name,
			Classname: collectionOf(sr.RequestID),
			Time:      junitTime(sr.DurationMs),
		}
		if c.Name == "" {
			c.Name = sr.RequestID
		}
		if sr.Skipped {
			c.Skipped = &junitSkipped{Message: "skipped (stop-on-failure)"}
		} else if msg, failed := junitFailureMessage(sr); failed {
			c.Failure = &junitFailure{Message: msg, Body: msg}
		}
		if sr.Skipped {
			suite.Skipped++
		}
		suite.Cases = append(suite.Cases, c)
	}
	b, err := xml.MarshalIndent(&suite, "", "  ")
	if err != nil {
		// Marshaling plain structs with string fields cannot fail in
		// practice; fall back to a minimal valid document.
		b, _ = xml.Marshal(&junitSuite{Name: "rapi"})
	}
	return append([]byte(xml.Header), b...)
}

// collectionOf returns the first path segment of a request id (the
// collection slug), or the id itself when it has no slash.
func collectionOf(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] == '/' {
			return id[:i]
		}
	}
	if id == "" {
		return "rapi"
	}
	return id
}

// junitFailureMessage picks the primary failure reason for a step: transport
// error, HTTP status >= 400, script error, then first failed assertion —
// the same precedence runner.stepFailed uses to mark a step failed. ok is
// false when the step passed.
func junitFailureMessage(sr StepResult) (msg string, ok bool) {
	res := sr.Result
	if res == nil {
		return "no result", true
	}
	switch {
	case res.Error != "":
		return res.Error, true
	case res.Status >= 400:
		code := strconv.Itoa(res.Status)
		if res.StatusText == "" {
			return "HTTP " + code, true
		}
		if strings.Contains(res.StatusText, code) {
			return "HTTP " + res.StatusText, true // StatusText is the full line
		}
		return "HTTP " + code + " " + res.StatusText, true
	case res.ScriptError != "":
		return "script error: " + res.ScriptError, true
	}
	for _, a := range res.Assertions {
		if a.Pass {
			continue
		}
		m := "assertion failed: " + a.Expression + " " + a.Operator
		if a.Expected != "" {
			m += " " + a.Expected
		}
		if a.Actual != "" {
			m += " (actual: " + a.Actual + ")"
		}
		return m, true
	}
	return "", false
}
