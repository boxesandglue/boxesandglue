package document

import "testing"

func TestParseFormat(t *testing.T) {
	cases := []struct {
		in     string
		want   Format
		wantOK bool
	}{
		{"", Format{}, true},
		{"PDF", Format{}, true},
		{"pdf", Format{}, true},
		{"PDF/A-3b", FormatPDFA3b, true},
		{"  pdf/a-3B  ", FormatPDFA3b, true},
		{"PDF/X-3", FormatPDFX3, true},
		{"PDF/X-4", FormatPDFX4, true},
		{"PDF/UA", FormatPDFUA, true},
		{"PDF/UA-1", FormatPDFUA, true},
		{"PDF/UA-2", FormatPDFUA2, true},
		{"PDF/A-3b, PDF/UA-1", Format{PDFA: &PDFAConf{3, PDFALevelB}, PDFUA: &PDFUAConf{1, "2014"}}, true},
		{"PDF/UA-2,PDF/X-4", Format{PDFX: &PDFXConf{"X-4"}, PDFUA: &PDFUAConf{2, "2024"}}, true},
		{"PDF/Z-99", Format{}, false},
		{"PDF/A-3b, gibberish", Format{}, false},
	}
	for _, c := range cases {
		got, err := ParseFormat(c.in)
		if c.wantOK && err != nil {
			t.Errorf("ParseFormat(%q): unexpected error: %v", c.in, err)
			continue
		}
		if !c.wantOK {
			if err == nil {
				t.Errorf("ParseFormat(%q): want error, got %v", c.in, got)
			}
			continue
		}
		if got.IsPDFA() != c.want.IsPDFA() || got.IsPDFUA() != c.want.IsPDFUA() || got.IsPDFX() != c.want.IsPDFX() {
			t.Errorf("ParseFormat(%q): sub-conformance presence mismatch: got %+v, want %+v", c.in, got, c.want)
		}
		if got.IsPDFA() && (got.PDFA.Part != c.want.PDFA.Part || got.PDFA.Level != c.want.PDFA.Level) {
			t.Errorf("ParseFormat(%q): PDFA mismatch: got %+v, want %+v", c.in, got.PDFA, c.want.PDFA)
		}
		if got.IsPDFUA() && (got.PDFUA.Part != c.want.PDFUA.Part || got.PDFUA.Rev != c.want.PDFUA.Rev) {
			t.Errorf("ParseFormat(%q): PDFUA mismatch: got %+v, want %+v", c.in, got.PDFUA, c.want.PDFUA)
		}
		if got.IsPDFX() && got.PDFX.Variant != c.want.PDFX.Variant {
			t.Errorf("ParseFormat(%q): PDFX mismatch: got %+v, want %+v", c.in, got.PDFX, c.want.PDFX)
		}
	}
}

func TestFormatStringRoundTrip(t *testing.T) {
	cases := []Format{
		FormatPDF,
		FormatPDFA3b,
		FormatPDFX3,
		FormatPDFX4,
		FormatPDFUA,
		FormatPDFUA2,
		{PDFA: &PDFAConf{3, PDFALevelB}, PDFUA: &PDFUAConf{1, "2014"}},
		{PDFA: &PDFAConf{3, PDFALevelB}, PDFUA: &PDFUAConf{2, "2024"}},
		{PDFX: &PDFXConf{"X-4"}, PDFUA: &PDFUAConf{2, "2024"}},
	}
	for _, c := range cases {
		s := c.String()
		got, err := ParseFormat(s)
		if err != nil {
			t.Errorf("round-trip of %+v via %q failed: %v", c, s, err)
			continue
		}
		if got.String() != s {
			t.Errorf("round-trip diverged: %q → %q", s, got.String())
		}
	}
}
