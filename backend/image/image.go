package image

import (
	pdf "github.com/speedata/baseline-pdf"
	"github.com/speedata/boxesandglue/backend/bag"
)

// Image represents a PDF file for inclusion.
type Image struct {
	PageNumber int // Requested page number
	Width      bag.ScaledPoint
	Height     bag.ScaledPoint
	ImageFile  *pdf.Imagefile
	Used       bool
}
