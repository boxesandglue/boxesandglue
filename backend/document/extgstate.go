package document

import (
	pdf "github.com/boxesandglue/baseline-pdf"
)

// materializeExtGStatesOnPage writes each requested graphics state as an
// indirect PDF object (cached document-wide by content inside the writer)
// and hooks it into the page's ExtGState resource dict. Called from the
// page output path when a rule carries an "extgstates" attribute; the
// rule's Pre code references the state as "/<ResourceName> gs".
func materializeExtGStatesOnPage(page *pdf.Page, doc *PDFDocument, items []pdf.ExtGState) error {
	if len(items) == 0 {
		return nil
	}
	if page.ExtGStates == nil {
		page.ExtGStates = make(map[pdf.Name]*pdf.Object)
	}
	for _, gs := range items {
		obj, err := doc.PDFWriter.WriteExtGState(gs)
		if err != nil {
			return err
		}
		page.ExtGStates[gs.ResourceName()] = obj
	}
	return nil
}
