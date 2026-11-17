package bidi

import (
	"slices"
	"testing"
)

type wantRun struct {
	text  string
	level uint8
}

func runsOf(t *testing.T, s string, opts ...Option) (Ordering, []wantRun) {
	t.Helper()
	var p Paragraph
	if _, err := p.SetString(s, opts...); err != nil {
		t.Fatal(err)
	}
	o, err := p.Order()
	if err != nil {
		t.Fatal(err)
	}
	var runs []wantRun
	for i := 0; i < o.NumRuns(); i++ {
		r := o.Run(i)
		runs = append(runs, wantRun{r.String(), r.Level()})
		wantDir := LeftToRight
		if r.Level()%2 == 1 {
			wantDir = RightToLeft
		}
		if r.Direction() != wantDir {
			t.Errorf("run %q: direction %v at level %d", r.String(), r.Direction(), r.Level())
		}
	}
	return o, runs
}

// TestRunLevels checks that runs keep the resolved levels. A number inside
// right-to-left text in a left-to-right paragraph has level 2 and forms a run
// of its own (boxesandglue#108).
func TestRunLevels(t *testing.T) {
	testcases := []struct {
		name string
		text string
		opts []Option
		runs []wantRun
	}{
		{
			name: "number in RTL run in LTR paragraph",
			text: "abc אבג 123 דהו xyz",
			opts: []Option{DefaultDirection(LeftToRight)},
			runs: []wantRun{{"abc ", 0}, {"אבג ", 1}, {"123", 2}, {" דהו", 1}, {" xyz", 0}},
		},
		{
			name: "same direction, different levels",
			text: "אבג 123abc",
			opts: []Option{DefaultDirection(LeftToRight)},
			runs: []wantRun{{"אבג ", 1}, {"123", 2}, {"abc", 0}},
		},
		{
			name: "LTR in RTL paragraph",
			text: "אבג abc 123 דהו",
			opts: []Option{DefaultDirection(RightToLeft)},
			runs: []wantRun{{"אבג ", 1}, {"abc 123", 2}, {" דהו", 1}},
		},
		{
			name: "explicit isolate",
			text: "abc ⁧אבג ⁦xyz⁩⁩ def",
			opts: []Option{DefaultDirection(LeftToRight)},
			runs: []wantRun{{"abc ⁧", 0}, {"אבג ⁦", 1}, {"xyz", 2}, {"⁩", 1}, {"⁩ def", 0}},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, runs := runsOf(t, tc.text, tc.opts...)
			if !slices.Equal(runs, tc.runs) {
				t.Errorf("runs %q, want %q", runs, tc.runs)
			}
		})
	}
}

// TestParagraphLevel checks rules P2 and P3: the first strong character sets
// the paragraph level, a leading number does not.
func TestParagraphLevel(t *testing.T) {
	testcases := []struct {
		text string
		opts []Option
		want uint8
	}{
		{"abc", nil, 0},
		{"אבג", nil, 1},
		{"2026 שנה טובה", nil, 1},
		{"123 abc", nil, 0},
		{"⁧abc⁩ אבג", nil, 1}, // isolates are skipped
		{"123", nil, 0},
		{"123", []Option{DefaultDirection(RightToLeft)}, 1},
		{"abc", []Option{DefaultDirection(RightToLeft)}, 1},
	}
	for _, tc := range testcases {
		o, _ := runsOf(t, tc.text, tc.opts...)
		if got := o.ParagraphLevel(); got != tc.want {
			t.Errorf("%q: paragraph level %d, want %d", tc.text, got, tc.want)
		}
	}
	for _, tc := range []struct {
		text string
		want Direction
	}{
		{"abc", LeftToRight},
		{"2026 שנה", RightToLeft},
		{"123 !", Neutral},
		{"⁧abc⁩", Neutral},
		{"⁧abc⁩ אבג", RightToLeft},
		{"⁧abc", Neutral}, // an isolate without PDI runs to the end
	} {
		var p Paragraph
		p.SetString(tc.text)
		if got := p.FirstStrong(); got != tc.want {
			t.Errorf("%q: FirstStrong %v, want %v", tc.text, got, tc.want)
		}
	}
	// The first run of the RTL paragraph is the number, left to right.
	o, _ := runsOf(t, "2026 שנה")
	if o.Direction() != LeftToRight {
		t.Errorf("first run direction %v, want LeftToRight", o.Direction())
	}
}

func TestLevels(t *testing.T) {
	o, _ := runsOf(t, "abc אבג 123 דהו xyz", DefaultDirection(LeftToRight))
	want := []uint8{0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1, 0, 0, 0, 0}
	if got := o.Levels(); !slices.Equal(got, want) {
		t.Errorf("levels %v, want %v", got, want)
	}
}

// TestCanonicalBrackets pairs the angle brackets U+2329/U+232A with their
// canonical equivalents U+3008/U+3009 (BD16).
func TestCanonicalBrackets(t *testing.T) {
	for _, s := range []string{"a 〈b.1〉", "a 〈b.1〉", "a 〈b.1〉", "a 〈b.1〉"} {
		o, _ := runsOf(t, s, DefaultDirection(RightToLeft))
		want := []uint8{2, 2, 2, 2, 2, 2, 2}
		if got := o.Levels(); !slices.Equal(got, want) {
			t.Errorf("%q: levels %v, want %v", s, got, want)
		}
	}
}

// TestOptionsReset checks that a default direction does not carry over to
// the next text of the same Paragraph.
func TestOptionsReset(t *testing.T) {
	var p Paragraph
	p.SetString("123", DefaultDirection(RightToLeft))
	if o, _ := p.Order(); o.ParagraphLevel() != 1 {
		t.Fatalf("paragraph level %d, want 1", o.ParagraphLevel())
	}
	p.SetString("123")
	if o, _ := p.Order(); o.ParagraphLevel() != 0 {
		t.Errorf("paragraph level %d after SetString without options, want 0", o.ParagraphLevel())
	}
}

// TestParagraphSeparator checks that a paragraph ends after its separator:
// SetString reports the bytes up to and including it, the separator gets the
// paragraph level, and the rest of the text is not part of the runs.
func TestParagraphSeparator(t *testing.T) {
	text := "abc אבג\nxyz"
	var p Paragraph
	n, err := p.SetString(text, DefaultDirection(LeftToRight))
	if err != nil {
		t.Fatal(err)
	}
	if want := len("abc אבג\n"); n != want {
		t.Errorf("SetString consumed %d bytes, want %d", n, want)
	}
	o, _ := p.Order()
	if got, want := o.Levels(), []uint8{0, 0, 0, 0, 1, 1, 1, 0}; !slices.Equal(got, want) {
		t.Errorf("levels %v, want %v", got, want)
	}
	if last := o.Run(o.NumRuns() - 1); last.String() != "\n" || last.Level() != 0 {
		t.Errorf("last run %q at level %d, want %q at level 0", last.String(), last.Level(), "\n")
	}
	n2, _ := p.SetString(text[n:], DefaultDirection(LeftToRight))
	if n2 != len("xyz") {
		t.Errorf("second paragraph: consumed %d bytes, want %d", n2, len("xyz"))
	}
}

// TestInvalidUTF8 checks that the consumed length counts the bytes of the
// input, not of the replacement characters.
func TestInvalidUTF8(t *testing.T) {
	var p Paragraph
	text := "a\xffb\nc"
	n, _ := p.SetString(text)
	if want := len("a\xffb\n"); n != want {
		t.Errorf("SetString consumed %d bytes, want %d", n, want)
	}
}

func TestRunAt(t *testing.T) {
	var p Paragraph
	p.SetString("abc אבג xyz", DefaultDirection(LeftToRight))
	p.Order()
	for _, tc := range []struct {
		pos  int
		want string
	}{{0, "abc "}, {3, "abc "}, {4, "אבג"}, {6, "אבג"}, {7, " xyz"}, {10, " xyz"}} {
		r := p.RunAt(tc.pos)
		if r.String() != tc.want {
			t.Errorf("RunAt(%d) = %q, want %q", tc.pos, r.String(), tc.want)
		}
	}
}
