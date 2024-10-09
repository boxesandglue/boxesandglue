package document

import pdf "github.com/boxesandglue/baseline-pdf"

// GetNumDest returns the PDF destination object with the internal number.
func (pw *PDFDocument) GetNumDest(num int) NumDest {
	return pw.numDestinations[num]
}

// NumDest represents a simple PDF destination. The origin of X and Y are in the
// top left corner and expressed in DTP points.
type NumDest struct {
	PageObjectnumber pdf.Objectnumber
	Num              int
	X                float64
	Y                float64
	objectnumber     pdf.Objectnumber
}
