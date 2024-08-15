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
		t.Errorf(err.Error())
	}
	nf, err := ff.GetFontSource(FontWeight400, FontStyleNormal)
	if err != nil {
		t.Errorf(err.Error())
	}
	if want, got := f, nf; want != got {
		t.Errorf("ff.GetFace() = %s, want %s", got, want)
	}
}
