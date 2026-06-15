package document

import (
	"fmt"
	"strings"
)

// ParseFormat parses a comma-separated list of PDF conformance-class
// names into a Format value. Recognised tokens (case-insensitive,
// whitespace-trimmed):
//
//	""        / "PDF"      → no claim (empty Format)
//	"PDF/A-3b"             → PDF/A-3 conformance level B
//	"PDF/X-3"              → PDF/X-3
//	"PDF/X-4"              → PDF/X-4
//	"PDF/UA"  / "PDF/UA-1" → PDF/UA-1
//	"PDF/UA-2"             → PDF/UA-2
//
// Multiple tokens compose: "PDF/A-3b, PDF/UA-1" declares both sub-
// conformances on the resulting Format. Unknown tokens return an error
// without partially populating the Format.
func ParseFormat(s string) (Format, error) {
	var f Format
	if strings.TrimSpace(s) == "" {
		return f, nil
	}
	for _, raw := range strings.Split(s, ",") {
		tok := strings.TrimSpace(raw)
		switch strings.ToUpper(tok) {
		case "", "PDF":
			// no claim — ignore extra "PDF" tokens
		case "PDF/A-3B":
			f.PDFA = &PDFAConf{Part: 3, Level: PDFALevelB}
		case "PDF/X-3":
			f.PDFX = &PDFXConf{Variant: "X-3"}
		case "PDF/X-4":
			f.PDFX = &PDFXConf{Variant: "X-4"}
		case "PDF/UA", "PDF/UA-1":
			f.PDFUA = &PDFUAConf{Part: 1, Rev: "2014"}
		case "PDF/UA-2":
			f.PDFUA = &PDFUAConf{Part: 2, Rev: "2024"}
		default:
			return Format{}, fmt.Errorf("unknown PDF format %q (recognised: PDF, PDF/A-3b, PDF/X-3, PDF/X-4, PDF/UA-1, PDF/UA-2)", tok)
		}
	}
	return f, nil
}

// String returns a canonical comma-separated representation of the
// declared sub-conformances. Round-trips through ParseFormat. Empty
// Format yields "PDF".
func (f Format) String() string {
	var parts []string
	if f.PDFA != nil {
		parts = append(parts, fmt.Sprintf("PDF/A-%d%s", f.PDFA.Part, strings.ToLower(string(f.PDFA.Level))))
	}
	if f.PDFX != nil {
		parts = append(parts, "PDF/"+f.PDFX.Variant)
	}
	if f.PDFUA != nil {
		parts = append(parts, fmt.Sprintf("PDF/UA-%d", f.PDFUA.Part))
	}
	if len(parts) == 0 {
		return "PDF"
	}
	return strings.Join(parts, ", ")
}
