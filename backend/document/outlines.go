package document

import "github.com/speedata/boxesandglue/pdfbackend/pdf"

// GetDest returns the PDF destination object with the internal number.
func (pw *PDFDocument) GetDest(num int) *pdf.Dest {
	return pw.PDFWriter.Destinations[num]
}
