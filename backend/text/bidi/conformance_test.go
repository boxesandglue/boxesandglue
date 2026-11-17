package bidi

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The test files of the Unicode Character Database for the version of the
// tables, gzipped: https://www.unicode.org/Public/17.0.0/ucd/BidiTest.txt and
// https://www.unicode.org/Public/17.0.0/ucd/BidiCharacterTest.txt. Replace
// them together with the tables.

// maxReported limits the failures a conformance test prints.
const maxReported = 10

func openTestFile(t *testing.T, name string) *bufio.Scanner {
	t.Helper()
	f, err := os.Open("testdata/" + name + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	firstLine := fmt.Sprintf("# %s-%s.txt", strings.TrimSuffix(name, ".txt"), UnicodeVersion)
	if !sc.Scan() || sc.Text() != firstLine {
		t.Fatalf("%s: first line %q, want %q (test file and tables must have the same Unicode version)", name, sc.Text(), firstLine)
	}
	return sc
}

// levelsMatch compares resolved levels against the levels of a test file,
// where x marks a character removed by rule X9 whose level is not tested.
func levelsMatch(got []uint8, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i, w := range want {
		if w == "x" {
			continue
		}
		n, err := strconv.Atoi(w)
		if err != nil || int(got[i]) != n {
			return false
		}
	}
	return true
}

// TestBidiCharacterTest resolves every line of BidiCharacterTest.txt through
// the public API: the paragraph level and the level of each character.
func TestBidiCharacterTest(t *testing.T) {
	sc := openTestFile(t, "BidiCharacterTest.txt")
	var cases, failures int
	for lineno := 2; sc.Scan(); lineno++ {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		// code points; paragraph direction; resolved paragraph level;
		// resolved levels; visual order
		fields := strings.Split(line, ";")
		if len(fields) != 5 {
			t.Fatalf("line %d: %d fields", lineno, len(fields))
		}
		var b strings.Builder
		for _, cp := range strings.Fields(fields[0]) {
			r, err := strconv.ParseUint(cp, 16, 32)
			if err != nil {
				t.Fatalf("line %d: %v", lineno, err)
			}
			b.WriteRune(rune(r))
		}
		var opts []Option
		switch fields[1] {
		case "0":
			opts = append(opts, DefaultDirection(LeftToRight))
		case "1":
			opts = append(opts, DefaultDirection(RightToLeft))
		}
		cases++
		var p Paragraph
		if _, err := p.SetString(b.String(), opts...); err != nil {
			t.Fatalf("line %d: %v", lineno, err)
		}
		o, err := p.Order()
		if err != nil {
			t.Fatalf("line %d: %v", lineno, err)
		}
		wantPara, _ := strconv.Atoi(fields[2])
		if fields[1] == "2" {
			// Without a default direction P2 and P3 decide, and FirstStrong
			// has to agree with them.
			fs := p.FirstStrong()
			if (fs == RightToLeft) != (wantPara == 1) {
				failures++
				if failures <= maxReported {
					t.Errorf("line %d: %s: FirstStrong %v, want paragraph level %d", lineno, fields[0], fs, wantPara)
				}
				continue
			}
		}
		gotLevels := o.Levels()
		if int(o.ParagraphLevel()) == wantPara && levelsMatch(gotLevels, strings.Fields(fields[3])) {
			continue
		}
		failures++
		if failures <= maxReported {
			t.Errorf("line %d: %s direction %s: paragraph level %d, levels %v; want %d, %s",
				lineno, fields[0], fields[1], o.ParagraphLevel(), gotLevels, wantPara, fields[3])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if failures > 0 {
		t.Errorf("%d of %d cases fail", failures, cases)
	}
}

var classNames = map[string]Class{
	"L": L, "R": R, "EN": EN, "ES": ES, "ET": ET, "AN": AN, "CS": CS,
	"B": B, "S": S, "WS": WS, "ON": ON, "BN": BN, "NSM": NSM, "AL": AL,
	"LRO": LRO, "RLO": RLO, "LRE": LRE, "RLE": RLE, "PDF": PDF,
	"LRI": LRI, "RLI": RLI, "FSI": FSI, "PDI": PDI,
}

// TestBidiTest resolves every line of BidiTest.txt. The file gives sequences
// of bidi classes instead of characters, so they go to the paragraph
// resolution directly. It has no brackets.
func TestBidiTest(t *testing.T) {
	sc := openTestFile(t, "BidiTest.txt")
	var want []string
	var cases, failures int
	for lineno := 2; sc.Scan(); lineno++ {
		line := sc.Text()
		switch {
		case line == "" || line[0] == '#':
			continue
		case strings.HasPrefix(line, "@Levels:"):
			want = strings.Fields(strings.TrimPrefix(line, "@Levels:"))
			continue
		case line[0] == '@':
			// @Reorder: the visual order, not tested here.
			continue
		}
		// classes; bitset of paragraph levels (1 auto, 2 LTR, 4 RTL)
		fields := strings.Split(line, ";")
		if len(fields) != 2 {
			t.Fatalf("line %d: %d fields", lineno, len(fields))
		}
		var types []Class
		for _, name := range strings.Fields(fields[0]) {
			c, ok := classNames[name]
			if !ok {
				t.Fatalf("line %d: unknown class %q", lineno, name)
			}
			types = append(types, c)
		}
		bits, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil {
			t.Fatalf("line %d: %v", lineno, err)
		}
		for bit, paraLevel := range []level{implicitLevel, 0, 1} {
			if bits&(1<<bit) == 0 {
				continue
			}
			cases++
			para, err := newParagraph(types, make([]bracketType, len(types)), make([]rune, len(types)), paraLevel)
			if err != nil {
				t.Fatalf("line %d: %v", lineno, err)
			}
			levels := para.getLevels([]int{len(types)})
			got := make([]uint8, len(levels))
			for i, l := range levels {
				got[i] = uint8(l)
			}
			if levelsMatch(got, want) {
				continue
			}
			failures++
			if failures <= maxReported {
				t.Errorf("line %d: %s paragraph level %d: levels %v, want %v", lineno, fields[0], paraLevel, got, want)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if failures > 0 {
		t.Errorf("%d of %d cases fail", failures, cases)
	}
}
