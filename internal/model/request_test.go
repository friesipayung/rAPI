package model

// Roundtrip tests for KV file items (P0 task 7.1): multipart form rows can
// carry Type "file" with a filesystem path in Value and a display name in
// FileName; old documents without the new fields must keep loading.

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKVFileItemRoundtrip(t *testing.T) {
	src := `type: multipart-form
form:
  - name: note
    value: hello
  - name: upload
    type: file
    value: /tmp/attachments/report.pdf
    fileName: report.pdf
`
	var body Body
	if err := yaml.Unmarshal([]byte(src), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.FormData) != 2 {
		t.Fatalf("form rows = %d", len(body.FormData))
	}
	file := body.FormData[1]
	if file.Type != "file" || file.Value != "/tmp/attachments/report.pdf" || file.FileName != "report.pdf" {
		t.Fatalf("file item fields lost: %+v", file)
	}
	if body.FormData[0].Type != "" || body.FormData[0].FileName != "" {
		t.Fatalf("text item polluted: %+v", body.FormData[0])
	}

	out, err := yaml.Marshal(&body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Body
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if len(back.FormData) != 2 {
		t.Fatalf("form rows after roundtrip = %d", len(back.FormData))
	}
	got := back.FormData[1]
	if got.Type != "file" || got.Value != "/tmp/attachments/report.pdf" || got.FileName != "report.pdf" {
		t.Fatalf("file item fields lost after roundtrip: %+v", got)
	}
}

func TestBodyWithoutFileFieldsLoads(t *testing.T) {
	// Fixture in the pre-7.1 shape: no type/fileName on form rows.
	src := `type: multipart-form
form:
  - name: user
    value: ali
  - name: extra
    value: "1"
    disabled: true
`
	var body Body
	if err := yaml.Unmarshal([]byte(src), &body); err != nil {
		t.Fatalf("legacy fixture failed to load: %v", err)
	}
	if len(body.FormData) != 2 || body.FormData[0].Name != "user" || body.FormData[0].Value != "ali" {
		t.Fatalf("legacy fixture decoded wrong: %+v", body.FormData)
	}
	for _, f := range body.FormData {
		if f.Type != "" || f.FileName != "" {
			t.Fatalf("unexpected new fields on legacy rows: %+v", f)
		}
	}
}
