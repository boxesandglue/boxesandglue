package frontend

import (
	"maps"
	"sync"
	"unicode"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/textshape/ot"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

// shapeFontFor resolves a FontSource to the (font, size, features, variations)
// tuple needed for HarfBuzz shaping. Mirrors the primary-font setup at the top
// of BuildNodelistFromString so coverage-driven runs use the same code path.
// CSS size-adjust, font-source-default features, and per-source variation
// settings are all applied here; the caller supplies base features (typically
// the document defaults) and per-call setting features (from
// SettingOpenTypeFeature) / variations (from SettingFontVariationSettings).
//
// Registers the resulting face in fe.usedFonts so PDF subsetting picks up
// glyphs emitted via the returned font — same as the primary path.
func (fe *Document) shapeFontFor(
	fs *FontSource,
	fontsize bag.ScaledPoint,
	baseFeatures []ot.Feature,
	settingFeatures []ot.Feature,
	settingVariations map[string]float64,
) (*font.Font, bag.ScaledPoint, []ot.Feature, map[string]float64, error) {
	if fs.SizeAdjust != 0 {
		fontsize = bag.ScaledPointFromFloat(fontsize.ToPT() * (1 - fs.SizeAdjust))
	}
	features := make([]ot.Feature, 0, len(baseFeatures)+len(settingFeatures)+4)
	features = append(features, baseFeatures...)
	features = append(features, parseOpenTypeFeatures(fs.FontFeatures)...)
	features = append(features, settingFeatures...)
	var variations map[string]float64
	if fs.VariationSettings != nil {
		variations = make(map[string]float64, len(fs.VariationSettings))
		maps.Copy(variations, fs.VariationSettings)
	}
	if settingVariations != nil {
		if variations == nil {
			variations = make(map[string]float64, len(settingVariations))
		}
		maps.Copy(variations, settingVariations)
	}
	face, err := fe.LoadFaceWithVariations(fs, variations)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	if fe.usedFonts[face] == nil {
		fe.usedFonts[face] = make(map[bag.ScaledPoint]*font.Font)
	}
	fnt, found := fe.usedFonts[face][fontsize]
	if !found {
		fnt = font.NewFont(face, fontsize)
		fnt.MissingGlyphFunc = fe.MissingGlyphFunc
		fe.usedFonts[face][fontsize] = fnt
	}
	return fnt, fontsize, features, variations, nil
}

// coverageRun is one contiguous, grapheme-cluster-aligned slice of source text
// resolved to a single FontSource. Per-glyph fallback shapes each run
// independently with the run's font and concatenates the atoms in run order;
// preserving cluster boundaries is what keeps ZWJ sequences (e.g. 👨‍👩‍👧)
// from being split across two faces.
type coverageRun struct {
	Text   string      // grapheme-cluster-aligned slice of the input
	Source *FontSource // resolved source; nil only when no stack entry covers any cluster in the run
	Family *FontFamily // origin family (stack[StackIndex])
	// StackIndex into the stack; 0 marks the primary. Whitespace clusters
	// always pin to 0 so inter-word glue keeps the primary's space metric.
	StackIndex int
}

// fontCoverageCache caches per-FontSource cmap probes. The probe is the hot
// path of segmentation; uncached HasGlyph re-parses the cmap table on every
// call. Cache lives per *Document because *FontSource pointers are scoped to
// a single document.
type fontCoverageCache struct {
	mu      sync.RWMutex
	entries map[*FontSource]map[ot.Codepoint]bool
}

func (c *fontCoverageCache) lookup(fs *FontSource, cp ot.Codepoint) (has bool, cached bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.entries == nil {
		return false, false
	}
	m, ok := c.entries[fs]
	if !ok {
		return false, false
	}
	has, ok = m[cp]
	return has, ok
}

func (c *fontCoverageCache) store(fs *FontSource, cp ot.Codepoint, has bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[*FontSource]map[ot.Codepoint]bool{}
	}
	m, ok := c.entries[fs]
	if !ok {
		m = map[ot.Codepoint]bool{}
		c.entries[fs] = m
	}
	m[cp] = has
}

// hasGlyph reports whether the FontSource covers cp. The face is loaded on
// demand because the coverage probe may be the first use of a fallback
// face; cmap parsing is cheap and the per-source cache amortises repeat
// lookups.
func (fe *Document) hasGlyph(fs *FontSource, cp ot.Codepoint) bool {
	if fs == nil {
		return false
	}
	if has, cached := fe.coverageCache.lookup(fs, cp); cached {
		return has
	}
	face, err := fe.LoadFace(fs)
	if err != nil || face == nil {
		fe.coverageCache.store(fs, cp, false)
		return false
	}
	otFace := face.OTFace()
	if otFace == nil || otFace.Font == nil {
		fe.coverageCache.store(fs, cp, false)
		return false
	}
	has := otFace.Font.HasGlyph(cp)
	fe.coverageCache.store(fs, cp, has)
	return has
}

// resolveClusterSource picks the first family in stack whose resolved source
// covers EVERY non-ignorable codepoint in cluster. Variation Selectors
// (U+FE0E/U+FE0F) bias the search but never veto: a font that lacks the VS
// itself is still acceptable as long as it covers the base character — VS
// glyphs are typically unencoded and the renderer keys color/text-presentation
// off the VS in the source string anyway.
//
// Returns the resolved source plus the stack index. A nil source means no
// stack entry covered the cluster — caller falls back to the primary so a
// .notdef glyph is at least visible (parity with current single-family
// behaviour where any uncovered codepoint goes straight to .notdef).
func (fe *Document) resolveClusterSource(cluster string, stack []*FontFamily, weight FontWeight, style FontStyle) (*FontSource, int) {
	// Strip variation selectors and other default-ignorables for the
	// coverage decision; we keep them in the cluster string because shape
	// time still consumes them (HarfBuzz handles VS internally).
	probeRunes := make([]rune, 0, len(cluster))
	for _, r := range cluster {
		if isCoverageIgnorable(r) {
			continue
		}
		probeRunes = append(probeRunes, r)
	}
	if len(probeRunes) == 0 {
		// All-ignorable cluster (rare: standalone VS, ZWJ). Stay on primary.
		fs, _ := stack[0].GetFontSource(weight, style)
		return fs, 0
	}
	for i, ff := range stack {
		fs, err := ff.GetFontSource(weight, style)
		if err != nil || fs == nil {
			continue
		}
		ok := true
		for _, r := range probeRunes {
			if !fe.hasGlyph(fs, ot.Codepoint(r)) {
				ok = false
				break
			}
		}
		if ok {
			return fs, i
		}
	}
	// No coverage: pin to primary so the .notdef stays attached to the
	// originally-intended face (matches single-family behaviour exactly).
	if len(stack) > 0 {
		fs, _ := stack[0].GetFontSource(weight, style)
		return fs, 0
	}
	return nil, 0
}

// isCoverageIgnorable returns true for codepoints that should not veto a
// font's coverage decision. Default-ignorable codepoints (Unicode UAX#44
// Default_Ignorable_Code_Point) are the strict spec definition; we
// additionally always-pass ZWJ (U+200D), ZWNJ (U+200C), and the two text /
// emoji presentation selectors (U+FE0E / U+FE0F). HarfBuzz removes them at
// shape time and they would otherwise force every emoji cluster to find a
// font that has glyphs for invisible format characters.
func isCoverageIgnorable(r rune) bool {
	switch r {
	case 0x200C, 0x200D, 0xFE0E, 0xFE0F:
		return true
	}
	// Variation Selectors block (U+FE00..U+FE0F + U+E0100..U+E01EF).
	if r >= 0xFE00 && r <= 0xFE0F {
		return true
	}
	if r >= 0xE0100 && r <= 0xE01EF {
		return true
	}
	// Default-ignorable formatting controls per UAX#44, restricted to the
	// commonly-occurring ranges; full table-driven coverage is overkill
	// for the per-glyph fallback path.
	if r == 0x00AD { // SOFT HYPHEN — handled separately, never gate coverage on it.
		return true
	}
	if unicode.Is(unicode.Cf, r) {
		return true
	}
	return false
}

// coverageSegments splits s into runs along grapheme-cluster boundaries
// (UAX#29 via clipperhouse/uax29) where each run is mapped to a single
// FontSource. Adjacent clusters resolving to the same source are merged so
// each run becomes one shape() call.
//
// Whitespace clusters pin to the primary (stack[0]) regardless of coverage —
// CSS spec says inter-word advance is the primary's; otherwise tab/space
// metrics would jitter at face boundaries.
//
// Returns nil if stack is empty or has only the primary; the caller should
// check `len(stack) >= 2` BEFORE entering coverage so single-family inputs
// take the unchanged single-shape path.
func (fe *Document) coverageSegments(s string, stack []*FontFamily, weight FontWeight, style FontStyle) []coverageRun {
	if len(stack) == 0 || s == "" {
		return nil
	}
	var runs []coverageRun
	g := graphemes.FromString(s)
	for g.Next() {
		cluster := g.Value()
		var src *FontSource
		var idx int
		if isWhitespaceCluster(cluster) {
			fs, _ := stack[0].GetFontSource(weight, style)
			src, idx = fs, 0
		} else {
			src, idx = fe.resolveClusterSource(cluster, stack, weight, style)
		}
		if n := len(runs); n > 0 && runs[n-1].Source == src && runs[n-1].StackIndex == idx {
			runs[n-1].Text += cluster
			continue
		}
		runs = append(runs, coverageRun{
			Text:       cluster,
			Source:     src,
			Family:     stack[idx],
			StackIndex: idx,
		})
	}
	return runs
}

// shapeForBuild is the multi-family-aware shape orchestrator called by
// BuildNodelistFromString. It returns the same (atoms, levels) pair as
// shapeWithBidi plus a parallel atomFonts slice naming which font produced
// each atom. The atom loop sets node.Glyph.Font from atomFonts[i] so per-glyph
// fallback survives without changing the atom struct.
//
// When fontfamilyStack has fewer than two entries the function takes the
// single-shape path and atomFonts is uniformly the primary font.
//
// Soft-hyphen sentinel atoms (Components == "­") carry the primary font so
// the Disc branch picks the primary's Hyphenchar — fallback faces typically
// lack a sensible hyphenchar, and a face switch at the line break would
// produce a visually jarring discretionary hyphen.
//
// Kerning is dropped at run boundaries (last atom's Kernafter = 0): cross-face
// kerning is undefined and the existing reverseAtoms shift logic only handles
// kerning within a single shape() call.
func (fe *Document) shapeForBuild(
	primaryFnt *font.Font,
	str string,
	primaryFeatures []ot.Feature,
	primaryVariations map[string]float64,
	direction Direction,
	stack []*FontFamily,
	weight FontWeight,
	style FontStyle,
	fontsize bag.ScaledPoint,
	baseFeatures []ot.Feature,
	settingFeatures []ot.Feature,
	settingVariations map[string]float64,
) ([]font.Atom, []uint8, []*font.Font) {
	if len(stack) < 2 {
		atoms, levels := shapeWithBidi(primaryFnt, str, primaryFeatures, primaryVariations, direction)
		atomFonts := make([]*font.Font, len(atoms))
		for i := range atomFonts {
			atomFonts[i] = primaryFnt
		}
		return atoms, levels, atomFonts
	}
	runs := fe.coverageSegments(str, stack, weight, style)
	var atoms []font.Atom
	var levels []uint8
	var atomFonts []*font.Font
	for _, run := range runs {
		runFnt := primaryFnt
		runFeatures := primaryFeatures
		runVariations := primaryVariations
		if run.StackIndex > 0 && run.Source != nil {
			rf, _, rfeat, rvar, err := fe.shapeFontFor(run.Source, fontsize, baseFeatures, settingFeatures, settingVariations)
			if err == nil && rf != nil {
				runFnt = rf
				runFeatures = rfeat
				runVariations = rvar
			}
		}
		runAtoms, runLevels := shapeWithBidi(runFnt, run.Text, runFeatures, runVariations, direction)
		if n := len(runAtoms); n > 0 {
			// Cross-face kerning is undefined; clip the trailing pair so
			// the last glyph of this segment doesn't pull the next.
			runAtoms[n-1].Kernafter = 0
		}
		for i, a := range runAtoms {
			atoms = append(atoms, a)
			levels = append(levels, runLevels[i])
			switch {
			case a.Components == "­":
				atomFonts = append(atomFonts, primaryFnt)
			case a.IsSpace:
				// Whitespace metric must come from the primary even if the
				// run resolved to a fallback, so inter-word advance stays
				// stable across face boundaries. coverageSegments already
				// pins whitespace clusters to the primary; this is a
				// belt-and-suspenders override at the atom layer.
				atomFonts = append(atomFonts, primaryFnt)
			default:
				atomFonts = append(atomFonts, runFnt)
			}
		}
	}
	return atoms, levels, atomFonts
}

// isWhitespaceCluster matches the common-case single-rune whitespace clusters
// that should pin to the primary font. Multi-rune clusters that happen to
// contain whitespace (rare) follow the normal coverage path — they are
// probably meaningful sequences (e.g. a ZWJ-glued composition).
func isWhitespaceCluster(cluster string) bool {
	if len(cluster) == 0 {
		return false
	}
	rs := []rune(cluster)
	if len(rs) != 1 {
		return false
	}
	return unicode.IsSpace(rs[0])
}
