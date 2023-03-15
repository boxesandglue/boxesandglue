package document

import "github.com/speedata/boxesandglue/pdfbackend/pdf"

// GetNumDest returns the PDF destination object with the internal number.
func (pw *PDFDocument) GetNumDest(num int) *pdf.NumDest {
	return pw.PDFWriter.NumDestinations[num]
}
