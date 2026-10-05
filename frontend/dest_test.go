package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A child Text with SettingDest, such as an inline element with an id, gets
// its destination in the paragraph, once, next to the paragraph's own.
func TestChildTextDestination(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("test")
	if err := ff.AddMember(
		&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"},
		FontWeight400, FontStyleNormal,
	); err != nil {
		t.Fatal(err)
	}
	child := NewText()
	child.Settings[SettingDest] = "s"
	child.Items = append(child.Items, "span")
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Settings[SettingDest] = "p"
	te.Items = append(te.Items, "A paragraph with a ", child, ".")

	for pass := 1; pass <= 2; pass++ {
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
		if err != nil {
			t.Fatal(err)
		}
		dests := map[any]int{}
		var walk func(n node.Node)
		walk = func(n node.Node) {
			for ; n != nil; n = n.Next() {
				switch v := n.(type) {
				case *node.StartStop:
					if v.Action == node.ActionDest {
						dests[v.Value]++
					}
				case *node.HList:
					walk(v.List)
				case *node.VList:
					walk(v.List)
				}
			}
		}
		walk(vl)
		if dests["p"] != 1 || dests["s"] != 1 || len(dests) != 2 {
			t.Errorf("pass %d: destinations %v, want p and s once each", pass, dests)
		}
	}
}
