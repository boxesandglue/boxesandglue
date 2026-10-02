package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// ruleBox is a VList holding a 2cm by 1cm rule.
func ruleBox() *node.VList {
	r := node.NewRule()
	r.Width = bag.MustSP("2cm")
	r.Height = bag.MustSP("1cm")
	return node.Vpack(node.Hpack(r))
}

// renderObject places vl at x, y on one page and returns the uncompressed
// page content stream.
func renderObject(t *testing.T, x, y bag.ScaledPoint, vl *node.VList) string {
	t.Helper()
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.CompressLevel = 0
	p := d.NewPage()
	p.OutputAt(x, y, vl)
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	at := strings.Index(out, " re f")
	if at < 0 {
		t.Fatalf("no rule in the output:\n%s", out)
	}
	start := strings.LastIndex(out[:at], "stream")
	end := at + strings.Index(out[at:], "endstream")
	return out[start:end]
}

// OutputAt moves a VList by its ShiftX and Shift, as a parent list does.
func TestOutputAtHonorsShift(t *testing.T) {
	x, y := bag.MustSP("3cm"), bag.MustSP("10cm")
	dx, dy := bag.MustSP("1cm"), bag.MustSP("-2cm")

	shifted := ruleBox()
	shifted.ShiftX = dx
	shifted.Shift = dy
	got := renderObject(t, x, y, shifted)
	want := renderObject(t, x+dx, y+dy, ruleBox())
	if got != want {
		t.Errorf("a shifted box is drawn as\n%s\nwant it drawn as one placed at the shifted position\n%s", got, want)
	}
	if unshifted := renderObject(t, x, y, ruleBox()); got == unshifted {
		t.Error("the shift has no effect")
	}
}
