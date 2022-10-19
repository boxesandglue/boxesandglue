package frontend

import (
	"os"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/document"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
	"github.com/speedata/textlayout/harfbuzz"
)

// Document holds convenience functions.
type Document struct {
	FontFamilies    map[string]*FontFamily
	Doc             *document.PDFDocument
	DefaultFeatures []harfbuzz.Feature
	FindFile        func(string) string
	usedcolors      map[string]*color.Color
	usedSpotcolors  map[*color.Color]bool
	usedFonts       map[*pdf.Face]map[bag.ScaledPoint]*font.Font
	dirstack        []string
}

func initDocument() *Document {
	d := &Document{
		usedSpotcolors: make(map[*color.Color]bool),
		usedcolors:     make(map[string]*color.Color),
		usedFonts:      make(map[*pdf.Face]map[bag.ScaledPoint]*font.Font),
		FontFamilies:   make(map[string]*FontFamily),
	}
	d.FindFile = d.findFile
	return d
}

// New creates a new PDF file. After Doc.Finish() is called, the file is closed.
func New(filename string) (*Document, error) {
	w, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	fe := initDocument()
	fe.Doc = document.NewDocument(w)
	fe.Doc.Filename = filename
	return fe, nil
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
