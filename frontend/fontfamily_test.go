package frontend

import (
	"io"
	"testing"
)

func Test(t *testing.T) {
	f := &FontSource{}
	ff := &FontFamily{}
	ff.doc, _ = initDocument(io.Discard)
	err := ff.AddMember(f, FontWeight400, FontStyleNormal)
	if err != nil {
		t.Error(err)
	}
	nf, err := ff.GetFontSource(FontWeight400, FontStyleNormal)
	if err != nil {
		t.Error(err)
	}
	if want, got := f, nf; want != got {
		t.Errorf("ff.GetFace() = %s, want %s", got, want)
	}
}

// TestRangeMember verifies CSS Fonts 4 weight-range members: containment
// resolves to a per-weight instance with the wght axis pinned, instances
// are cached by identity, point members win at their exact weight, and
// out-of-range requests clamp to the nearest range end.
func TestRangeMember(t *testing.T) {
	vf := &FontSource{Name: "VF"}
	ff := &FontFamily{}
	ff.doc, _ = initDocument(io.Discard)
	if err := ff.AddMemberRange(vf, 200, 900, FontStyleNormal); err != nil {
		t.Fatal(err)
	}

	inst, err := ff.GetFontSource(500, FontStyleNormal)
	if err != nil {
		t.Fatal(err)
	}
	if inst == vf {
		t.Error("expected a derived instance, got the base font source")
	}
	if got := inst.VariationSettings["wght"]; got != 500 {
		t.Errorf("wght = %v, want 500", got)
	}

	inst2, err := ff.GetFontSource(500, FontStyleNormal)
	if err != nil {
		t.Fatal(err)
	}
	if inst != inst2 {
		t.Error("instance at the same weight must be pointer-identical")
	}

	// Out of range: clamp to the nearest end.
	heavy, err := ff.GetFontSource(950, FontStyleNormal)
	if err != nil {
		t.Fatal(err)
	}
	if got := heavy.VariationSettings["wght"]; got != 900 {
		t.Errorf("wght = %v, want 900 (clamped)", got)
	}

	// A point member at its exact weight beats the range.
	static := &FontSource{Name: "Static700"}
	if err := ff.AddMember(static, 700, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	got, err := ff.GetFontSource(700, FontStyleNormal)
	if err != nil {
		t.Fatal(err)
	}
	if got != static {
		t.Errorf("point member must win at its exact weight, got %s", got)
	}

	// Base variation settings are carried over, wght is overridden.
	vfItalic := &FontSource{Name: "VFItalic", VariationSettings: map[string]float64{"slnt": -10, "wght": 400}}
	if err := ff.AddMemberRange(vfItalic, 200, 900, FontStyleItalic); err != nil {
		t.Fatal(err)
	}
	it, err := ff.GetFontSource(600, FontStyleItalic)
	if err != nil {
		t.Fatal(err)
	}
	if got := it.VariationSettings["slnt"]; got != -10 {
		t.Errorf("slnt = %v, want -10 (inherited)", got)
	}
	if got := it.VariationSettings["wght"]; got != 600 {
		t.Errorf("wght = %v, want 600 (overrides base)", got)
	}
}
