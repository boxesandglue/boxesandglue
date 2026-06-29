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

// ParseFormatAxes builds a Format from three orthogonal axis values, one
// per conformance family. This is the structured counterpart to the
// comma-list ParseFormat: each family is selected independently, which
// makes "two from the same family" unrepresentable. Empty string or
// "none" (case-insensitive) means the axis is not claimed.
//
//	pdfua: "", "none", "1" (PDF/UA-1), "2" (PDF/UA-2)
//	pdfa:  "", "none", "3b" (PDF/A-3 level B)
//	pdfx:  "", "none", "X-3", "X-4"
//
// Recognised values mirror what the PDF writer can actually emit; values
// outside the set return an error. Cross-axis combinations that demand
// incompatible base PDF versions (e.g. PDF/UA-2 wants PDF 2.0, PDF/A-3 and
// PDF/X want PDF 1.x) are rejected too.
func ParseFormatAxes(pdfua, pdfa, pdfx string) (Format, error) {
	var f Format
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

	switch norm(pdfua) {
	case "", "none":
	case "1", "ua-1":
		f.PDFUA = &PDFUAConf{Part: 1, Rev: "2014"}
	case "2", "ua-2":
		f.PDFUA = &PDFUAConf{Part: 2, Rev: "2024"}
	default:
		return Format{}, fmt.Errorf("unknown pdfua value %q (recognised: none, 1, 2)", pdfua)
	}

	switch norm(pdfa) {
	case "", "none":
	case "3b", "a-3b":
		f.PDFA = &PDFAConf{Part: 3, Level: PDFALevelB}
	default:
		return Format{}, fmt.Errorf("unknown pdfa value %q (recognised: none, 3b)", pdfa)
	}

	switch norm(pdfx) {
	case "", "none":
	case "x-3", "3":
		f.PDFX = &PDFXConf{Variant: "X-3"}
	case "x-4", "4":
		f.PDFX = &PDFXConf{Variant: "X-4"}
	default:
		return Format{}, fmt.Errorf("unknown pdfx value %q (recognised: none, X-3, X-4)", pdfx)
	}

	// Cross-axis base-version coupling: PDF/UA-2 mandates PDF 2.0, while
	// PDF/A-1..3 and PDF/X-3/4 are PDF 1.x. Claiming both at once is
	// self-contradictory; pair UA-2 with a PDF 2.0 conformance instead.
	if f.PDFUA != nil && f.PDFUA.Part == 2 {
		if f.PDFA != nil {
			return Format{}, fmt.Errorf("PDF/UA-2 (PDF 2.0) cannot be combined with PDF/A-%d (PDF 1.7)", f.PDFA.Part)
		}
		if f.PDFX != nil {
			return Format{}, fmt.Errorf("PDF/UA-2 (PDF 2.0) cannot be combined with PDF/%s (PDF 1.x)", f.PDFX.Variant)
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
