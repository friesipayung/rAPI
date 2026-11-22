package runner

// Unit test for the JUnit rendering (P0 task 3.2): the XML must parse, the
// counts must match the report (1 pass + 1 fail + 1 skipped), skipped steps
// carry <skipped/>, failed steps carry <failure message=...>, and special
// characters in names/messages must survive a round trip unharmed.

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

// rawSuite/rawCase parse the produced XML generically (attr lists + child
// element names only) so the assertions check the actual document structure
// instead of re-using the writer's own structs.
type rawSuite struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Cases   []rawCase  `xml:"testcase"`
}

type rawCase struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Inner []rawInner `xml:",any"`
}

type rawInner struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
}

func (e rawInner) attr(name string) string {
	for _, a := range e.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (c rawCase) attr(name string) string {
	for _, a := range c.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (s rawSuite) attr(name string) string {
	for _, a := range s.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func sampleReport() *Report {
	return &Report{
		StartedAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC),
		TotalMs:   1250,
		EnvUsed:   "dev",
		FailCount: 1,
		Steps: []StepResult{
			{Step: Step{RequestID: "chain/login", Name: "Login"}, Result: &model.RunResult{Status: 200}, DurationMs: 100},
			{Step: Step{RequestID: "chain/echo", Name: "Echo"}, Result: &model.RunResult{Status: 500, StatusText: "500 Internal Server Error"}, DurationMs: 150},
			{Step: Step{RequestID: "chain/late", Name: "Late"}, Skipped: true},
		},
	}
}

func TestMarshalJunitCounts(t *testing.T) {
	data := sampleReport().MarshalJunit()

	if !strings.HasPrefix(string(data), xml.Header) {
		t.Errorf("output must start with the XML declaration, got %q", string(data[:min(60, len(data))]))
	}

	var suite rawSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("output must parse as XML: %v\n%s", err, data)
	}
	if suite.XMLName.Local != "testsuite" {
		t.Errorf("root element = %q, want testsuite", suite.XMLName.Local)
	}
	if got := suite.attr("tests"); got != "3" {
		t.Errorf("tests attr = %q, want 3", got)
	}
	if got := suite.attr("failures"); got != "1" {
		t.Errorf("failures attr = %q, want 1", got)
	}
	if got := suite.attr("skipped"); got != "1" {
		t.Errorf("skipped attr = %q, want 1", got)
	}
	if got := suite.attr("time"); got != "1.250" {
		t.Errorf("time attr = %q, want 1.250", got)
	}
	if len(suite.Cases) != 3 {
		t.Fatalf("got %d testcase elements, want 3", len(suite.Cases))
	}

	// pass: no failure/skipped child, classname groups by collection slug
	if n := len(suite.Cases[0].Inner); n != 0 {
		t.Errorf("passing case must have no child elements, got %d", n)
	}
	if got := suite.Cases[0].attr("classname"); got != "chain" {
		t.Errorf("classname = %q, want chain", got)
	}
	if got := suite.Cases[0].attr("time"); got != "0.100" {
		t.Errorf("case time = %q, want 0.100", got)
	}

	// fail: exactly one <failure message=...> child
	if len(suite.Cases[1].Inner) != 1 || suite.Cases[1].Inner[0].XMLName.Local != "failure" {
		t.Errorf("failed case must carry a <failure> child, got %+v", suite.Cases[1].Inner)
	}
	msg := suite.Cases[1].Inner[0].attr("message")
	if !strings.Contains(msg, "500") {
		t.Errorf("failure message must mention the status, got %q", msg)
	}

	// skipped: exactly one <skipped/> child
	if len(suite.Cases[2].Inner) != 1 || suite.Cases[2].Inner[0].XMLName.Local != "skipped" {
		t.Errorf("skipped case must carry a <skipped/> child, got %+v", suite.Cases[2].Inner)
	}
}

func TestMarshalJunitEscaping(t *testing.T) {
	nasty := `He said <"run"> & ` + "`exit`"
	rep := &Report{
		Steps: []StepResult{
			{Step: Step{RequestID: "c/x", Name: nasty}, Result: &model.RunResult{Error: `dial tcp: <broken> & more`}, DurationMs: 1},
		},
		FailCount: 1,
	}
	data := rep.MarshalJunit()

	// Round trip through the parser: if the encoder failed to escape, the
	// document would be ill-formed or the values would come back mangled.
	var suite rawSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("escaping produced invalid XML: %v\n%s", err, data)
	}
	if got := suite.Cases[0].attr("name"); got != nasty {
		t.Errorf("name round trip = %q, want %q", got, nasty)
	}
	if got := suite.Cases[0].Inner[0].attr("message"); got != `dial tcp: <broken> & more` {
		t.Errorf("failure message round trip = %q", got)
	}
	if !strings.Contains(string(data), "&lt;") {
		t.Errorf("raw output must escape < as &lt;, got:\n%s", data)
	}
}

func TestMarshalJunitEmpty(t *testing.T) {
	data := (&Report{}).MarshalJunit()
	var suite rawSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("empty report must still produce valid XML: %v\n%s", err, data)
	}
	if got := suite.attr("tests"); got != "0" {
		t.Errorf("tests attr = %q, want 0", got)
	}
}
