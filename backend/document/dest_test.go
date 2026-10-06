package document

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A destination in a line sits at the top of the line box, where a
// destination before the line in the vertical list sits.
func TestDestinationInALineAtTheLineTop(t *testing.T) {
	d := NewDocument(io.Discard)
	p := d.NewPage()

	dest := func(name string) *node.StartStop {
		ss := node.NewStartStop()
		ss.Action = node.ActionDest
		ss.Value = name
		return ss
	}
	r := node.NewRule()
	r.Width = bag.MustSP("2cm")
	r.Height = bag.MustSP("8pt")
	r.Depth = bag.MustSP("3pt")
	hl := node.Hpack(node.InsertBefore(r, r, dest("inline")))
	vl := node.Vpack(node.InsertBefore(hl, hl, dest("vertical")))

	x, y := bag.MustSP("3cm"), bag.MustSP("10cm")
	p.OutputAt(x, y, vl)
	p.Shipout()

	got := map[string]float64{}
	for name, nd := range d.PDFWriter.NameDestinations {
		got[string(name)] = nd.Y
	}
	if got["vertical"] != y.ToPT() {
		t.Fatalf("vertical destination at y %v, want %v", got["vertical"], y.ToPT())
	}
	if got["inline"] != got["vertical"] {
		t.Errorf("destination in the line at y %v, want the line top %v", got["inline"], got["vertical"])
	}
}
