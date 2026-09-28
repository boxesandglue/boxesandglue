package node

import "testing"

// TestGlueCopy checks that a copy keeps every field of the glue, including
// what makes it a tab with a leader.
func TestGlueCopy(t *testing.T) {
	g := NewGlue()
	g.Width = 10
	g.Stretch = 2
	g.Shrink = 1
	g.StretchOrder = StretchFil
	g.ShrinkOrder = StretchFill
	g.Subtype = GlueTab
	g.Leader = NewHList()
	g.LeaderType = LeaderCentered
	c := g.Copy().(*Glue)
	if c.Width != 10 || c.Stretch != 2 || c.Shrink != 1 || c.StretchOrder != StretchFil || c.ShrinkOrder != StretchFill {
		t.Errorf("copy %v plus %v minus %v (orders %d, %d), want 10 plus 2 minus 1 (orders 1, 2)", c.Width, c.Stretch, c.Shrink, c.StretchOrder, c.ShrinkOrder)
	}
	if c.Subtype != GlueTab {
		t.Errorf("copy subtype %d, want GlueTab", c.Subtype)
	}
	if c.Leader != g.Leader || c.LeaderType != LeaderCentered {
		t.Errorf("copy leader %v (type %d), want the original's pattern, centered", c.Leader, c.LeaderType)
	}
}
