package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// TestWhiteSpaceAxes guards the two decisions the property makes at this
// level. Whether whitespace collapses is settled earlier, when text is
// extracted from the markup; what is left here is the space's width and
// whether a line may break at it.
//
//   - keepsSpaceWidth: pre and pre-wrap render a space at its own glyph
//     advance. The rest use the font's inter-word default.
//   - breaksAtSpace: pre and nowrap do not wrap, which also means a soft
//     hyphen must not offer a break.
func TestWhiteSpaceAxes(t *testing.T) {
	cases := []struct {
		name      string
		in        WhiteSpace
		keepWidth bool
		breaks    bool
	}{
		{"normal", WhiteSpaceNormal, false, true},
		{"nowrap", WhiteSpaceNowrap, false, false},
		{"pre", WhiteSpacePre, true, false},
		{"pre-wrap", WhiteSpacePreWrap, true, true},
		{"pre-line", WhiteSpacePreLine, false, true},
	}
	for _, tc := range cases {
		if got := tc.in.keepsSpaceWidth(); got != tc.keepWidth {
			t.Errorf("%s: keepsSpaceWidth() = %v, want %v", tc.name, got, tc.keepWidth)
		}
		if got := tc.in.breaksAtSpace(); got != tc.breaks {
			t.Errorf("%s: breaksAtSpace() = %v, want %v", tc.name, got, tc.breaks)
		}
	}
}

// TestWhiteSpaceZeroValue pins the Go zero value to CSS's initial value, so a
// caller that never sets SettingWhiteSpace gets normal behaviour.
func TestWhiteSpaceZeroValue(t *testing.T) {
	var ws WhiteSpace
	if ws != WhiteSpaceNormal {
		t.Errorf("zero value = %v, want WhiteSpaceNormal", ws)
	}
}

// TestDecorationOffset checks each line sits where its name says: the
// underline below the baseline, the strike through the lowercase band, the
// overline clear of the ascender.
func TestDecorationOffset(t *testing.T) {
	const size = bag.ScaledPoint(10 * 65536)
	under := DecorationOffset(TextDecorationUnderline, size)
	strike := DecorationOffset(TextDecorationLineThrough, size)
	over := DecorationOffset(TextDecorationOverline, size)

	if under >= 0 {
		t.Errorf("underline offset = %v, want below the baseline", under)
	}
	if strike <= 0 || strike >= over {
		t.Errorf("line-through offset = %v, want between the baseline and the overline (%v)", strike, over)
	}
	if over <= 0 || over >= size {
		t.Errorf("overline offset = %v, want above the baseline and within the em", over)
	}
	if got := DecorationOffset(TextDecorationLineNone, size); got != 0 {
		t.Errorf("none offset = %v, want 0", got)
	}
}
