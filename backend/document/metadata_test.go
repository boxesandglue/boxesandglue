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

// The XMP values of dc:title and dc:creator are exactly the title and the
// author, without surrounding white space. Otherwise Acrobat's PDF/X-4
// preflight reports that the document properties and the XMP disagree.
func TestMetadataLanguageAlternativeValue(t *testing.T) {
	for _, format := range []string{"PDF/A-3b", "PDF/UA-1", "PDF/X-3", "PDF/X-4"} {
		f, err := ParseFormat(format)
		if err != nil {
			t.Fatal(err)
		}
		d := NewDocument(&bytes.Buffer{})
		d.Format = f
		d.Title = "A & B"
		d.Author = "C"
		var xmp bytes.Buffer
		d.getMetadata(&xmp)
		if !strings.Contains(xmp.String(), `<rdf:li xml:lang="x-default">A &amp; B</rdf:li>`) {
			t.Errorf("%s: title not written as the exact value in\n%s", format, xmp.String())
		}
		if f.IsPDFA() && !strings.Contains(xmp.String(), "<rdf:li>C</rdf:li>") {
			t.Errorf("%s: author not written as the exact value in\n%s", format, xmp.String())
		}
	}
}
