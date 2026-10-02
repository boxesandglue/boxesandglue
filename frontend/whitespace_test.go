package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
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

// TestStripLeadingTrailingGlueInSpans verifies that collapsible whitespace
// inside spans at paragraph edges is removed while StartStop markers,
// NBSP, and tabs are preserved.
func TestStripLeadingTrailingGlueInSpans(t *testing.T) {
	// 1. Span with leading and trailing spaces: | space glyph space |
	s1 := node.NewStartStop()
	gLead := node.NewGlue()
	glyph := node.NewGlyph()
	gTrail := node.NewGlue()
	s2 := node.NewStartStop()

	node.InsertAfter(s1, s1, gLead)
	node.InsertAfter(s1, gLead, glyph)
	node.InsertAfter(s1, glyph, gTrail)
	node.InsertAfter(s1, gTrail, s2)

	head, tail := stripLeadingTrailingGlue(s1, s2, false)
	if head != s1 || tail != s2 {
		t.Fatalf("head = %v, tail = %v; want s1, s2", head, tail)
	}
	if s1.Next() != glyph || glyph.Next() != s2 {
		t.Errorf("expected s1 -> glyph -> s2, got %v -> %v -> %v", s1, s1.Next(), s1.Next().Next())
	}

	// 2. Trailing space in a span alone: glyph | | space |
	g := node.NewGlyph()
	sA := node.NewStartStop()
	sB := node.NewStartStop()
	sp := node.NewGlue()
	sC := node.NewStartStop()

	node.InsertAfter(g, g, sA)
	node.InsertAfter(g, sA, sB)
	node.InsertAfter(g, sB, sp)
	node.InsertAfter(g, sp, sC)

	head, tail = stripLeadingTrailingGlue(g, sC, false)
	if head != g || tail != sC {
		t.Fatalf("head = %v, tail = %v; want g, sC", head, tail)
	}
	if sB.Next() != sC {
		t.Errorf("expected sp to be stripped between sB and sC, got %v", sB.Next())
	}

	// 3. NBSP inside span: | penalty(10000) glue glyph penalty(10000) glue |
	sOpen := node.NewStartStop()
	p1 := node.NewPenalty()
	p1.Penalty = 10000
	gNBSP1 := node.NewGlue()
	mid := node.NewGlyph()
	p2 := node.NewPenalty()
	p2.Penalty = 10000
	gNBSP2 := node.NewGlue()
	sClose := node.NewStartStop()

	node.InsertAfter(sOpen, sOpen, p1)
	node.InsertAfter(sOpen, p1, gNBSP1)
	node.InsertAfter(sOpen, gNBSP1, mid)
	node.InsertAfter(sOpen, mid, p2)
	node.InsertAfter(sOpen, p2, gNBSP2)
	node.InsertAfter(sOpen, gNBSP2, sClose)

	head, tail = stripLeadingTrailingGlue(sOpen, sClose, false)
	if head != sOpen || tail != sClose {
		t.Fatalf("head = %v, tail = %v; want sOpen, sClose", head, tail)
	}
	if p1.Next() != gNBSP1 || p2.Next() != gNBSP2 {
		t.Errorf("NBSP glue should not be stripped")
	}

	// 4. Tab stop inside span: | tab-glue glyph |
	sTabOpen := node.NewStartStop()
	tabGlue := node.NewGlue()
	tabGlue.Subtype = node.GlueTab
	tabGlyph := node.NewGlyph()
	sTabClose := node.NewStartStop()

	node.InsertAfter(sTabOpen, sTabOpen, tabGlue)
	node.InsertAfter(sTabOpen, tabGlue, tabGlyph)
	node.InsertAfter(sTabOpen, tabGlyph, sTabClose)

	head, tail = stripLeadingTrailingGlue(sTabOpen, sTabClose, true)
	if head != sTabOpen || tail != sTabClose {
		t.Fatalf("head = %v, tail = %v; want sTabOpen, sTabClose", head, tail)
	}
	if sTabOpen.Next() != tabGlue {
		t.Errorf("tab glue should not be stripped when keepTabs is true")
	}

	// 5. Padded span with StartStop: | s1 kern(pad-left) glue glyph glue kern(pad-right) s2 |
	sP1 := node.NewStartStop()
	kPL := node.NewKern()
	kPL.Attributes = node.H{"origin": "padding left"}
	kPL.Kern = bag.MustSP("5pt")
	gPLead := node.NewGlue()
	pGlyph := node.NewGlyph()
	gPTrail := node.NewGlue()
	kPR := node.NewKern()
	kPR.Attributes = node.H{"origin": "padding right"}
	kPR.Kern = bag.MustSP("5pt")
	sP2 := node.NewStartStop()

	node.InsertAfter(sP1, sP1, kPL)
	node.InsertAfter(sP1, kPL, gPLead)
	node.InsertAfter(sP1, gPLead, pGlyph)
	node.InsertAfter(sP1, pGlyph, gPTrail)
	node.InsertAfter(sP1, gPTrail, kPR)
	node.InsertAfter(sP1, kPR, sP2)

	head, tail = stripLeadingTrailingGlue(sP1, sP2, false)
	if head != sP1 || tail != sP2 {
		t.Fatalf("head = %v, tail = %v; want sP1, sP2", head, tail)
	}
	if sP1.Next() != kPL || kPL.Next() != pGlyph || pGlyph.Next() != kPR || kPR.Next() != sP2 {
		t.Errorf("expected sP1 -> kPL -> pGlyph -> kPR -> sP2, got %v -> %v -> %v -> %v -> %v",
			sP1, sP1.Next(), sP1.Next().Next(), sP1.Next().Next().Next(), sP1.Next().Next().Next().Next())
	}

	// 6. Padded span without StartStop: | kern(pad-left) glue glyph glue kern(pad-right) |
	kPL2 := node.NewKern()
	kPL2.Attributes = node.H{"origin": "padding left"}
	kPL2.Kern = bag.MustSP("5pt")
	gPLead2 := node.NewGlue()
	pGlyph2 := node.NewGlyph()
	gPTrail2 := node.NewGlue()
	kPR2 := node.NewKern()
	kPR2.Attributes = node.H{"origin": "padding right"}
	kPR2.Kern = bag.MustSP("5pt")

	node.InsertAfter(kPL2, kPL2, gPLead2)
	node.InsertAfter(kPL2, gPLead2, pGlyph2)
	node.InsertAfter(kPL2, pGlyph2, gPTrail2)
	node.InsertAfter(kPL2, gPTrail2, kPR2)

	head, tail = stripLeadingTrailingGlue(kPL2, kPR2, false)
	if head != kPL2 || tail != kPR2 {
		t.Fatalf("head = %v, tail = %v; want kPL2, kPR2", head, tail)
	}
	if kPL2.Next() != pGlyph2 || pGlyph2.Next() != kPR2 {
		t.Errorf("expected kPL2 -> pGlyph2 -> kPR2, got %v -> %v -> %v",
			kPL2, kPL2.Next(), kPL2.Next().Next())
	}
}

// TestPaddedSpanWhitespaceAtEdge verifies that padding kerns are preserved
// at paragraph edges while collapsible whitespace inside spans is stripped,
// across all four permutations of (with/without spaces) and (with/without background).
func TestPaddedSpanWhitespaceAtEdge(t *testing.T) {
	fe, ff := lineModelDocument(t)
	yellow := fe.GetColor("yellow")
	pad := bag.MustSP("5pt")

	cases := []struct {
		name       string
		text       string
		background bool
	}{
		{"with spaces, with background", " one ", true},
		{"without spaces, with background", "one", true},
		{"with spaces, without background", " one ", false},
		{"without spaces, without background", "one", false},
	}

	var bgWidthWithSpaces, bgWidthNoSpaces bag.ScaledPoint
	var plainWidthWithSpaces, plainWidthNoSpaces bag.ScaledPoint

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			span := NewText()
			if tc.background {
				span.Settings[SettingBackgroundColor] = yellow
			}
			span.Settings[SettingPaddingLeft] = pad
			span.Settings[SettingPaddingRight] = pad
			span.Items = append(span.Items, tc.text)

			te := NewText()
			te.Settings[SettingFontFamily] = ff
			te.Settings[SettingSize] = bag.MustSP("10pt")
			te.Items = append(te.Items, span)

			vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
			if err != nil {
				t.Fatalf("FormatParagraph: %v", err)
			}
			hl, ok := vl.List.(*node.HList)
			if !ok {
				t.Fatalf("expected *node.HList line, got %T", vl.List)
			}

			// Verify that padding kerns (5pt) are present at left and right
			var hasPadLeft, hasPadRight bool
			for n := hl.List; n != nil; n = n.Next() {
				if k, ok := n.(*node.Kern); ok {
					if origin, _ := k.Attributes["origin"].(string); origin == "padding left" && k.Kern == pad {
						hasPadLeft = true
					}
					if origin, _ := k.Attributes["origin"].(string); origin == "padding right" && k.Kern == pad {
						hasPadRight = true
					}
				}
			}
			if !hasPadLeft {
				t.Errorf("missing padding left kern of %v", pad)
			}
			if !hasPadRight {
				t.Errorf("missing padding right kern of %v", pad)
			}

			if tc.background {
				rules := backgroundRules(vl)
				if countRules(rules) != 1 {
					t.Fatalf("got %d background rules, want 1", countRules(rules))
				}
				if tc.text == " one " {
					bgWidthWithSpaces = rules[0][0].Width
				} else {
					bgWidthNoSpaces = rules[0][0].Width
				}
			} else {
				// Measure content width between left padding and right padding
				var spanStart, spanStop node.Node
				for n := hl.List; n != nil; n = n.Next() {
					if k, ok := n.(*node.Kern); ok {
						if origin, _ := k.Attributes["origin"].(string); origin == "padding left" {
							spanStart = n
						}
						if origin, _ := k.Attributes["origin"].(string); origin == "padding right" {
							spanStop = n
						}
					}
				}
				spanWidth, _, _ := node.Dimensions(spanStart, spanStop, node.Horizontal)
				if tc.text == " one " {
					plainWidthWithSpaces = spanWidth
				} else {
					plainWidthNoSpaces = spanWidth
				}
			}
		})
	}

	// Verify that stripped spaces do not widen the background box or span
	if bgWidthWithSpaces != bgWidthNoSpaces {
		t.Errorf("background box with spaces (%v) != without spaces (%v)", bgWidthWithSpaces, bgWidthNoSpaces)
	}
	if plainWidthWithSpaces != plainWidthNoSpaces {
		t.Errorf("span width with spaces (%v) != without spaces (%v)", plainWidthWithSpaces, plainWidthNoSpaces)
	}
}
