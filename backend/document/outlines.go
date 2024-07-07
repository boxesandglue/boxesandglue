package document

import pdf "github.com/boxesandglue/baseline-pdf"

// GetNumDest returns the PDF destination object with the internal number.
func (pw *PDFDocument) GetNumDest(num int) *pdf.NumDest {
	return pw.PDFWriter.NumDestinations[num]
}
