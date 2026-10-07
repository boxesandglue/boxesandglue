package document

import (
	"bytes"
	"strings"
	"testing"
)

// A document that is PDF/A-3 and PDF/UA at once declares the pdfuaid
// schema as an extension, which PDF/A-3 requires for it; one that is only
// either of them does not.
func TestMetadataDeclaresPDFUAIDForPDFA(t *testing.T) {
	const decl = "<pdfaSchema:prefix>pdfuaid</pdfaSchema:prefix>"
	for _, tc := range []struct {
		format string
		want   bool
	}{
		{"PDF/A-3b,PDF/UA-1", true},
		{"PDF/A-3b", false},
		{"PDF/UA-1", false},
	} {
		f, err := ParseFormat(tc.format)
		if err != nil {
			t.Fatal(err)
		}
		d := NewDocument(&bytes.Buffer{})
		d.Format = f
		var xmp bytes.Buffer
		d.getMetadata(&xmp)
		if got := strings.Contains(xmp.String(), decl); got != tc.want {
			t.Errorf("%s: pdfuaid declared %t, want %t in\n%s", tc.format, got, tc.want, xmp.String())
		}
	}
}
