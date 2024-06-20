package frontend

import (
	"io"
	"os"
	"time"

	pdf "github.com/speedata/baseline-pdf"
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/document"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/textlayout/harfbuzz"
)

// Document holds convenience functions.
type Document struct {
	FontFamilies          map[string]*FontFamily
	Doc                   *document.PDFDocument
	DefaultFeatures       []harfbuzz.Feature
	fontlocal             map[string]*FontSource
	suppressInfo          bool
	usedcolors            map[string]*color.Color
	usedSpotcolors        map[*color.Color]bool
	usedFonts             map[*pdf.Face]map[bag.ScaledPoint]*font.Font
	dirstack              []string
	postLinebreakCallback []PostLinebreakCallbackFunc
}

func initDocument() *Document {
	d := &Document{
		usedSpotcolors: make(map[*color.Color]bool),
		usedcolors:     make(map[string]*color.Color),
		usedFonts:      make(map[*pdf.Face]map[bag.ScaledPoint]*font.Font),
		FontFamilies:   make(map[string]*FontFamily),
		fontlocal:      make(map[string]*FontSource),
	}
	return d
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
	fe := initDocument()
	fe.Doc = document.NewDocument(w)
    if err := fe.RegisterCallback(CallbackPostLinebreak, PostLinebreakCallbackFunc(postLinebreak)); err != nil {
		return nil, err
	}
	return fe, nil
}

// SetSuppressInfo sets the suppressinfo flag.
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
)
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
