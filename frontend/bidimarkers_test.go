package frontend

import (
	"io"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// markerLine builds the line of a bidi marker test. A string item is a glyph
// with that label at the level that follows it in the same item ("א1"); a
// node is inserted as it is.
func markerLine(items ...any) *node.HList {
	var head, tail node.Node
	for _, itm := range items {
		var n node.Node
		switch t := itm.(type) {
		case string:
			g := node.NewGlyph()
			g.Components = t[:len(t)-1]
			g.SetBidiLevel(t[len(t)-1] - '0')
			n = g
		case node.Node:
			n = t
		}
		head = node.InsertAfter(head, tail, n)
		tail = n
	}
	hl := node.NewHList()
	hl.List = head
	return hl
}

// markerPair returns a start and a stop that points to it.
func markerPair() (*node.StartStop, *node.StartStop) {
	start := node.NewStartStop()
	start.Action = node.ActionHyperlink
	start.Value = "target"
	stop := node.NewStartStop()
	stop.StartNode = start
	return start, stop
}

// markerLabels reads a line back: a glyph as its label, a node from names as
// its name, any other StartStop as "start'" or "stop'" (a copy).
func markerLabels(hl *node.HList, names map[node.Node]string) string {
	var out []string
	for n := hl.List; n != nil; n = n.Next() {
		if name, ok := names[n]; ok {
			out = append(out, name)
			continue
		}
		switch t := n.(type) {
		case *node.Glyph:
			out = append(out, t.Components)
		case *node.StartStop:
			if t.StartNode == nil {
				out = append(out, "start'")
			} else {
				out = append(out, "stop'")
			}
		default:
			out = append(out, "?")
		}
	}
	return strings.Join(out, " ")
}

// TestBidiMarkersKeepRTLRun: a pair around a run in an RTL paragraph does not
// cut the run, and after the reorder the start still comes before the stop,
// around the same glyphs.
func TestBidiMarkersKeepRTLRun(t *testing.T) {
	start, stop := markerPair()
	hl := markerLine("א1", start, "ב1", "ג1", stop, "ד1")
	bidiReorderLine(hl, 1)
	names := map[node.Node]string{start: "start", stop: "stop"}
	if got, want := markerLabels(hl, names), "ד start ג ב stop א"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if stop.StartNode != start {
		t.Error("the stop no longer points to its start")
	}
}

// TestBidiMarkersLTRRunInRTL: a link around an English word in Arabic text
// keeps the word in its place and the link around it.
func TestBidiMarkersLTRRunInRTL(t *testing.T) {
	start, stop := markerPair()
	hl := markerLine("א1", start, "e2", "n2", stop, "ב1")
	bidiReorderLine(hl, 1)
	names := map[node.Node]string{start: "start", stop: "stop"}
	if got, want := markerLabels(hl, names), "ב start e n stop א"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestBidiMarkersSplitAtDirectionChange: a pair whose content is not one
// visual run gets a start and a stop around each part. The original start
// opens the part with its logical start, the original stop closes the part
// with its logical end, and each stop points to the start of its part.
func TestBidiMarkersSplitAtDirectionChange(t *testing.T) {
	start, stop := markerPair()
	hl := markerLine("a0", start, "b0", "א1", stop, "ב1")
	bidiReorderLine(hl, 0)
	names := map[node.Node]string{start: "start", stop: "stop"}
	if got, want := markerLabels(hl, names), "a start b stop' ב start' א stop"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	stopCopy, _ := start.Next().Next().(*node.StartStop)
	startCopy, _ := stop.Prev().Prev().(*node.StartStop)
	if stopCopy == nil || startCopy == nil {
		t.Fatal("no copies around the parts")
	}
	if stopCopy.StartNode != start {
		t.Error("the stop of the first part does not point to the original start")
	}
	if stop.StartNode != startCopy {
		t.Error("the original stop does not point to the start of its part")
	}
	if startCopy.Action != start.Action || startCopy.Value != start.Value {
		t.Error("the copied start lost the action or the value of the original")
	}
}

// TestBidiMarkersNested: where parts of two pairs meet, the inner stop comes
// before the outer one.
func TestBidiMarkersNested(t *testing.T) {
	outer, outerStop := markerPair()
	inner, innerStop := markerPair()
	hl := markerLine("a0", outer, "b0", inner, "א1", innerStop, "ב1", outerStop)
	bidiReorderLine(hl, 0)
	names := map[node.Node]string{outer: "outer", outerStop: "/outer", inner: "inner", innerStop: "/inner"}
	if got, want := markerLabels(hl, names), "a outer b ב inner א /inner /outer"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestBidiMarkersAcrossLines: a pair over a line break is closed at the end
// of the first line and opened again on the next, around the ink of each
// line: the spaces at the line edges stay outside.
func TestBidiMarkersAcrossLines(t *testing.T) {
	start, stop := markerPair()
	space := node.NewGlue()
	space.SetBidiLevel(1)
	line1 := markerLine("א1", start, "ב1", "ג1", space)
	space2 := node.NewGlue()
	space2.SetBidiLevel(1)
	line2 := markerLine(space2, "ד1", stop, "ה1")
	bidiReorderLines([]*node.HList{line1, line2}, 1)
	names := map[node.Node]string{start: "start", stop: "stop", space: "space", space2: "space2"}
	if got, want := markerLabels(line1, names), "space start ג ב stop' א"; got != want {
		t.Errorf("line 1: got %q, want %q", got, want)
	}
	if got, want := markerLabels(line2, names), "ה start' ד stop space2"; got != want {
		t.Errorf("line 2: got %q, want %q", got, want)
	}
	if s, ok := start.Next().Next().Next().(*node.StartStop); !ok || s.StartNode != start {
		t.Error("the stop on line 1 does not point to the start")
	}
	if stop.StartNode != line2.List.Next() {
		t.Error("the stop on line 2 does not point to the start on line 2")
	}
}

// TestBidiMarkersUnpaired: a marker without a partner, a destination, stays
// at the logical start of the glyph that follows it, its right edge in RTL
// text, and one at the end of the line at the left edge of the last glyph.
func TestBidiMarkersUnpaired(t *testing.T) {
	dest := node.NewStartStop()
	dest.Action = node.ActionDest
	lang := node.NewLang()
	end := node.NewStartStop()
	end.Action = node.ActionDest
	hl := markerLine("א1", dest, "ב1", lang, "ג1", end)
	bidiReorderLine(hl, 1)
	names := map[node.Node]string{dest: "dest", lang: "lang", end: "end"}
	if got, want := markerLabels(hl, names), "end ג lang ב dest א"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestBidiMarkersEmptyPair: a pair that encloses nothing, an empty link,
// stays together between the glyphs it was between.
func TestBidiMarkersEmptyPair(t *testing.T) {
	start, stop := markerPair()
	hl := markerLine("א1", start, stop, "ב1")
	bidiReorderLine(hl, 1)
	names := map[node.Node]string{start: "start", stop: "stop"}
	if got, want := markerLabels(hl, names), "ב start stop א"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestBidiMarkersLTRParagraphUntouched: a paragraph without anything to
// reorder keeps its nodes, even a pair over a line break.
func TestBidiMarkersLTRParagraphUntouched(t *testing.T) {
	start, stop := markerPair()
	line1 := markerLine("a0", start, "b0")
	line2 := markerLine("c0", stop, "d0")
	bidiReorderLines([]*node.HList{line1, line2}, 0)
	names := map[node.Node]string{start: "start", stop: "stop"}
	if got, want := markerLabels(line1, names)+" | "+markerLabels(line2, names), "a start b | c stop d"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestBidiMarkersInRTLParagraph sets an RTL paragraph with a background, a
// coloured span and a link (bag#84). Each marks its own word after the
// reorder, and the words keep the order of the paragraph without them.
func TestBidiMarkersInRTLParagraph(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("test")
	if err := ff.AddMember(&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	span := func(key SettingType, value any, s string) *Text {
		te := NewText()
		te.Settings[key] = value
		te.Items = append(te.Items, s)
		return te
	}
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Items = append(te.Items, "אב ",
		span(SettingBackgroundColor, fe.GetColor("yellow"), "גד"), " ",
		span(SettingColor, "red", "הו"), " ",
		span(SettingHyperlink, document.Hyperlink{URI: "https://example.com"}, "זח"), " טי")
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("300pt"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []*node.HList
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			lines = append(lines, hl)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	hl := lines[0]
	// What each marker encloses, read from left to right.
	var words []string
	enclosed := map[*node.StartStop]*strings.Builder{}
	var glyphs strings.Builder
	for n := hl.List; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Glyph:
			glyphs.WriteString(v.Components)
			for _, b := range enclosed {
				b.WriteString(v.Components)
			}
		case *node.Glue:
			glyphs.WriteString(" ")
		case *node.StartStop:
			if v.StartNode == nil {
				enclosed[v] = &strings.Builder{}
				continue
			}
			b, ok := enclosed[v.StartNode]
			if !ok {
				t.Fatalf("a stop before its start")
			}
			words = append(words, b.String())
			delete(enclosed, v.StartNode)
		}
	}
	if len(enclosed) != 0 {
		t.Fatalf("%d starts without a stop", len(enclosed))
	}
	if got, want := strings.TrimSpace(glyphs.String()), "יט חז וה דג בא"; got != want {
		t.Errorf("visual order %q, want %q", got, want)
	}
	if got, want := strings.Join(words, ","), "חז,וה,דג"; got != want {
		t.Errorf("marked words from left to right %q, want %q", got, want)
	}
	if rules := backgroundRules(vl); countRules(rules) != 1 {
		t.Errorf("got %d background rules, want 1", countRules(rules))
	} else {
		n := node.Node(rules[0][0])
		for ; n != nil; n = n.Next() {
			if _, ok := n.(*node.Glyph); ok {
				break
			}
		}
		if g, ok := n.(*node.Glyph); !ok || g.Components != "ד" {
			t.Errorf("the background does not start at the left of its word")
		}
	}
}
