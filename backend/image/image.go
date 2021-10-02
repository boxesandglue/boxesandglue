package image

import (
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
)

// Image represents a PDF file for inclusion.
type Image struct {
	PageNumber int
	ImageFile  *pdf.Imagefile
	Used       bool
}
