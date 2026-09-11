package frontend

import (
	"io"
	"testing"
)

// TestRangeInstancesDistinctFaces guards the face routing for weight-range
// members: every per-weight instance pins its own wght value, so two weights
// of the same font file must resolve to two faces. The document-level face
// loader dedups by filename only; handing it a variation-pinned source made
// all weights share one face whose VariationSettings the instances then
// overwrote in turn, collapsing the whole range to a single weight.
func TestRangeInstancesDistinctFaces(t *testing.T) {
	fe, err := initDocument(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("vf")
	ff.AddMemberRange(
		&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"},
		200, 900, FontStyleNormal,
	)

	fs400, err := ff.GetFontSource(400, FontStyleNormal)
	if err != nil {
		t.Fatal(err)
	}
	fs700, err := ff.GetFontSource(700, FontStyleNormal)
	if err != nil {
		t.Fatal(err)
	}

	f400, err := fe.LoadFace(fs400)
	if err != nil {
		t.Fatal(err)
	}
	f700, err := fe.LoadFace(fs700)
	if err != nil {
		t.Fatal(err)
	}
	if f400 == f700 {
		t.Fatal("two weights of the same range share one face")
	}
	if got := f400.VariationSettings["wght"]; got != 400 {
		t.Errorf("face for weight 400 pins wght=%v", got)
	}
	if got := f700.VariationSettings["wght"]; got != 700 {
		t.Errorf("face for weight 700 pins wght=%v", got)
	}

	// Same weight again: the instance and its face must be cached.
	fs400b, err := ff.GetFontSource(400, FontStyleNormal)
	if err != nil {
		t.Fatal(err)
	}
	f400b, err := fe.LoadFace(fs400b)
	if err != nil {
		t.Fatal(err)
	}
	if f400b != f400 {
		t.Error("repeated lookup of the same weight created a second face")
	}
}
