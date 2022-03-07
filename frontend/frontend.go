package frontend

import (
	"os"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/document"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
)

// Document holds convenience functions.
type Document struct {
	FontFamilies []*FontFamily
	Doc          *document.PDFDocument
	colors       map[string]*Color
	usedFonts    map[*pdf.Face]map[bag.ScaledPoint]*font.Font
}

// New creates a new PDF file. After Doc.Finish() is called, the file is closed.
func New(filename string) (*Document, error) {
	w, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	fe := &Document{
		Doc:       document.NewDocument(w),
		colors:    csscolors,
		usedFonts: make(map[*pdf.Face]map[bag.ScaledPoint]*font.Font),
	}
	fe.Doc.Filename = filename
	return fe, nil
}

// d.colors = csscolors
