package document

import (
	"strings"
	"testing"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestHboxTraceRule checks the hbox trace drawing: translucent height and
// depth areas plus baseline in transparent mode, outline and baseline only
// in the opaque fallback.
func TestHboxTraceRule(t *testing.T) {
	hl := node.NewHList()
	hl.Width = bag.MustSP("100pt")
	hl.Height = bag.MustSP("8pt")
	hl.Depth = bag.MustSP("2pt")

	r := hboxTraceRule(hl, true)
	if !r.Hide {
		t.Error("trace rule must be hidden (Pre-only)")
	}
	if !strings.Contains(r.Pre, "/GSca0_4 gs") {
		t.Errorf("transparent trace must activate the alpha ExtGState, got %q", r.Pre)
	}
	// height and depth fills plus outline and baseline strokes
	if got := strings.Count(r.Pre, " f"); got != 2 {
		t.Errorf("expected 2 fills (height, depth), got %d in %q", got, r.Pre)
	}
	if got := strings.Count(r.Pre, " S"); got != 2 {
		t.Errorf("expected 2 strokes (outline, baseline), got %d in %q", got, r.Pre)
	}
	if gss, ok := r.Attributes["extgstates"].([]pdf.ExtGState); !ok || len(gss) != 1 {
		t.Errorf("trace rule must request exactly one ExtGState, got %v", r.Attributes["extgstates"])
	}

	// A line without depth must not paint a zero-height depth area.
	hl.Depth = 0
	r = hboxTraceRule(hl, true)
	if got := strings.Count(r.Pre, " f"); got != 1 {
		t.Errorf("expected 1 fill for depth-less line, got %d in %q", got, r.Pre)
	}

	// Opaque fallback: no ExtGState, no fills.
	hl.Depth = bag.MustSP("2pt")
	r = hboxTraceRule(hl, false)
	if strings.Contains(r.Pre, " gs") {
		t.Errorf("opaque fallback must not use an ExtGState, got %q", r.Pre)
	}
	if strings.Contains(r.Pre, " f ") {
		t.Errorf("opaque fallback must not fill, got %q", r.Pre)
	}
	if _, ok := r.Attributes["extgstates"]; ok {
		t.Error("opaque fallback must not request an ExtGState")
	}
}
