package frontend

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// Document holds convenience functions.
type Document struct {
	FontFamilies          map[string]*FontFamily
	Doc                   *document.PDFDocument
	fontlocal             map[string]*FontSource
	usedcolors            map[string]*color.Color
	usedSpotcolors        map[*color.Color]bool
	usedFonts             map[*pdf.Face]map[bag.ScaledPoint]*font.Font
	variationFaces        map[string]*pdf.Face // cache for faces with specific variations
	DefaultFeatures       []ot.Feature
	MissingGlyphFunc      font.MissingGlyphFunc // Called when a character is not found in the font during shaping. If nil, missing glyphs are silently rendered as .notdef.
	coverageCache         fontCoverageCache     // per-FontSource cmap probe cache for per-glyph fallback; zero-value is valid
	dirstack              []string
	postLinebreakCallback []PostLinebreakCallbackFunc
	suppressInfo          bool
}

// missingGlyphKey deduplicates missing-glyph warnings: the same character
// missing from the same face fails on every occurrence, one report is enough.
type missingGlyphKey struct {
	face *pdf.Face
	r    rune
}

func initDocument(w io.Writer) (*Document, error) {
	d := &Document{
		usedSpotcolors: make(map[*color.Color]bool),
		usedcolors:     make(map[string]*color.Color),
		usedFonts:      make(map[*pdf.Face]map[bag.ScaledPoint]*font.Font),
		variationFaces: make(map[string]*pdf.Face),
		FontFamilies:   make(map[string]*FontFamily),
		fontlocal:      make(map[string]*FontSource),
		Doc:            document.NewDocument(w),
	}
	// A character without a glyph is rendered as .notdef (the tofu box) and
	// makes the PDF fail PDF/UA (Matterhorn 10-004), so it should never stay
	// silent. Warn once per face and rune: a code block full of box-drawing
	// characters would otherwise flood the log. This is only a default;
	// callers may replace MissingGlyphFunc or set it to nil for the previous
	// silent behaviour.
	var missingMu sync.Mutex
	missingSeen := make(map[missingGlyphKey]bool)
	d.MissingGlyphFunc = func(face *pdf.Face, r rune) {
		missingMu.Lock()
		defer missingMu.Unlock()
		key := missingGlyphKey{face: face, r: r}
		if missingSeen[key] {
			return
		}
		missingSeen[key] = true
		bag.Logger.Warn("Font has no glyph for character, shown as .notdef", "char", string(r), "codepoint", fmt.Sprintf("U+%04X", r), "font", face.PostscriptName)
	}
	// Honour the reproducible-builds.org SOURCE_DATE_EPOCH convention
	// at library level so every consumer (bagme, glu/markdown,
	// glu/.lua entry, ad-hoc callers) gets deterministic timestamps
	// and XMP UUIDs without having to re-implement the lookup.
	// Callers that explicitly assign Doc.CreationDate after this
	// still win — frontend.New returns to them before Finish.
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			d.Doc.CreationDate = time.Unix(secs, 0).UTC()
			d.Doc.SuppressInfo = true
		}
	}
	var err error
	if d.Doc.DefaultLanguage, err = GetLanguage("en"); err != nil {
		return nil, err
	}

	return d, nil
}

// New creates a new document writing to a new PDF file
// with the given filename. New DOES NOT close this file.
func New(filename string) (*Document, error) {
	w, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	fe, err := NewForWriter(w)
	if err != nil {
		return nil, err
	}

	fe.Doc.Filename = filename
	return fe, nil
}

// NewForWriter creates a new Document writing to w. w is never closed.
func NewForWriter(w io.Writer) (*Document, error) {
	fe, err := initDocument(w)
	if err != nil {
		return nil, err
	}
	if err = fe.RegisterCallback(CallbackPostLinebreak, PostLinebreakCallbackFunc(postLinebreak)); err != nil {
		return nil, err
	}
	return fe, nil
}

// SetSuppressInfo sets the suppressinfo flag. This tries to write reproducible
// PDF files by having the time stamp set to a fixed date.
func (fe *Document) SetSuppressInfo(si bool) {
	fe.suppressInfo = si
	fe.Doc.SuppressInfo = si
	if pdfCreationdate, err := time.Parse("2006-01-02", "2023-08-31"); err == nil {
		fe.Doc.CreationDate = pdfCreationdate
	}
}

// Finish writes all necessary objects for the PDF.
func (fe *Document) Finish() error {
	for col := range fe.usedSpotcolors {
		fe.Doc.Spotcolors = append(fe.Doc.Spotcolors, col)
	}
	if len(fe.usedSpotcolors) > 0 {
		if fe.Doc.ColorProfile == nil {
			_, err := fe.Doc.LoadDefaultColorprofile()
			if err != nil {
				return err
			}
		}
	}
	return fe.Doc.Finish()
}

// HangingPunctuation determines if the right or the left side should have
// hanging punctuation. Values should be or'ed together.
type HangingPunctuation uint8

const (
	// HangingPunctuationAllowEnd allows hanging punctuation at the end of a
	// line.
	HangingPunctuationAllowEnd = 1
)

// HorizontalAlignment is the horizontal alignment.
type HorizontalAlignment int

// VerticalAlignment is the vertical alignment.
type VerticalAlignment int

const (
	// HAlignDefault is an undefined alignment.
	HAlignDefault HorizontalAlignment = iota
	// HAlignLeft makes text ragged right.
	HAlignLeft
	// HAlignRight makes text ragged left.
	HAlignRight
	// HAlignCenter has ragged left and right alignment.
	HAlignCenter
	// HAlignJustified makes text left and right aligned.
	HAlignJustified
	// HAlignStart is the CSS Text 3 logical "start" alignment: left in
	// LTR paragraphs, right in RTL paragraphs. FormatParagraph resolves
	// it to HAlignLeft or HAlignRight after the paragraph direction is
	// known.
	HAlignStart
	// HAlignEnd is the CSS Text 3 logical "end" alignment: right in LTR
	// paragraphs, left in RTL paragraphs. FormatParagraph resolves it
	// like HAlignStart.
	HAlignEnd
)

func (ha HorizontalAlignment) String() string {
	switch ha {
	case HAlignDefault:
		return "default"
	case HAlignLeft:
		return "left"
	case HAlignRight:
		return "right"
	case HAlignCenter:
		return "center"
	case HAlignJustified:
		return "justified"
	case HAlignStart:
		return "start"
	case HAlignEnd:
		return "end"
	}
	return "---"
}

const (
	// VAlignDefault is an undefined vertical alignment.
	VAlignDefault VerticalAlignment = iota
	// VAlignTop aligns the contents at the top of the surrounding box.
	VAlignTop
	// VAlignMiddle aligns the contents in the vertical middle of the surrounding box.
	VAlignMiddle
	// VAlignBottom aligns the contents at the bottom of the surrounding box.
	VAlignBottom
)

// NewStructureElement creates a new structure element with the given role.
func (d *Document) NewStructureElement(role string) *document.StructureElement {
	return &document.StructureElement{Role: role}
}

// SetRootStructureElement sets the root structure element for the document.
func (d *Document) SetRootStructureElement(se *document.StructureElement) {
	d.Doc.RootStructureElement = se
}

// TagVList associates a VList with a structure element for PDF tagging.
func (d *Document) TagVList(vl *node.VList, se *document.StructureElement) {
	if vl.Attributes == nil {
		vl.Attributes = node.H{}
	}
	vl.Attributes["tag"] = se
}

// MarkAsArtifact marks a VList as a PDF artifact (decorative content).
func (d *Document) MarkAsArtifact(vl *node.VList, artifactType document.ArtifactType) {
	if vl.Attributes == nil {
		vl.Attributes = node.H{}
	}
	vl.Attributes["artifact"] = artifactType
}
