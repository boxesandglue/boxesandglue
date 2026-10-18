package document

import (
	"bytes"
	"regexp"
	"testing"
)

// A document-level attachment gets the /AFRelationship it asks for, and
// Alternative when it names none (#95).
func TestAttachmentAFRelationship(t *testing.T) {
	var out bytes.Buffer
	d := NewDocument(&out)
	d.NewPage().Shipout()
	d.AttachFile(Attachment{Name: "default.xml", MimeType: "text/xml", Data: []byte("<a/>")})
	d.AttachFile(Attachment{Name: "data.xml", MimeType: "text/xml", Data: []byte("<b/>"), AFRelationship: "Data"})
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	rel := regexp.MustCompile(`/AFRelationship /(\w+)`)
	name := regexp.MustCompile(`\n /F \(([^)]*)\)`)
	got := map[string]string{}
	for _, obj := range bytes.Split(out.Bytes(), []byte("endobj")) {
		if !bytes.Contains(obj, []byte("/Type /Filespec")) {
			continue
		}
		r, n := rel.FindSubmatch(obj), name.FindSubmatch(obj)
		if r == nil || n == nil {
			t.Fatalf("Filespec without /AFRelationship or /F:\n%s", obj)
		}
		got[string(n[1])] = string(r[1])
	}
	for file, want := range map[string]string{"default.xml": "Alternative", "data.xml": "Data"} {
		if got[file] != want {
			t.Errorf("%s: AFRelationship %q, want %q (found %v)", file, got[file], want, got)
		}
	}
}
