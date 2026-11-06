package document

import (
	pdf "github.com/boxesandglue/baseline-pdf"
)

// materializeExtGStates writes each requested graphics state as an indirect
// PDF object (cached document-wide by content inside the writer) and hooks it
// into the ExtGState resource dict of a page or a form. Called from the page
// output path when a rule carries an "extgstates" attribute; the rule's Pre
// code references the state as "/<ResourceName> gs".
func materializeExtGStates(res *pdf.Resources, doc *PDFDocument, items []pdf.ExtGState) error {
	if len(items) == 0 {
		return nil
	}
	if res.ExtGStates == nil {
		res.ExtGStates = make(map[pdf.Name]*pdf.Object)
	}
	for _, gs := range items {
		obj, err := doc.PDFWriter.WriteExtGState(gs)
		if err != nil {
			return err
		}
		res.ExtGStates[gs.ResourceName()] = obj
	}
	return nil
}
