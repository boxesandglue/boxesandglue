package image

import (
	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// Image represents a PDF file for inclusion.
type Image struct {
	PageNumber int // Requested page number
	Width      bag.ScaledPoint
	Height     bag.ScaledPoint
	ImageFile  *pdf.Imagefile
	Used       bool
}
