package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A VList in running text stands on the baseline of its line. The writer
// hangs a VList in an HList from the top of the HList, as the cells of a
// table row want; in a line that put a box lower than the line at its top.
// Mknodes wraps it in an HList of its size, which the writer places on the
// baseline.
func TestMknodesVListOnBaseline(t *testing.T) {
	fe, ff := lineModelDocument(t)
	box := node.NewVList()
	box.Width, box.Height, box.Depth = bag.MustSP("5pt"), bag.MustSP("4pt"), bag.MustSP("1pt")
	box.Attributes = node.H{"origin": "test"}
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("20pt")
	te.Items = append(te.Items, "x", box, "x")
	head, _, err := fe.Mknodes(te)
	if err != nil {
		t.Fatal(err)
	}
	for n := head; n != nil; n = n.Next() {
		if vl, ok := n.(*node.VList); ok {
			t.Fatalf("the VList %v is in the line as it is", vl.Attributes)
		}
		hl, ok := n.(*node.HList)
		if !ok {
			continue
		}
		vl, ok := hl.List.(*node.VList)
		if !ok {
			continue
		}
		if o, _ := vl.Attributes["origin"].(string); o != "test" {
			t.Errorf("wrapped VList has origin %q, want the copy of the box", o)
		}
		if hl.Width != box.Width || hl.Height != box.Height || hl.Depth != box.Depth {
			t.Errorf("wrapper %s×%s+%s, want the size of the box %s×%s+%s", hl.Width, hl.Height, hl.Depth, box.Width, box.Height, box.Depth)
		}
		if vl.Next() != nil {
			t.Error("the wrapper holds more than the box")
		}
		return
	}
	t.Fatal("no HList with the box in the line")
}
