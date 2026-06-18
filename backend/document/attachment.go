package document

import (
	"fmt"
	"time"

	pdf "github.com/boxesandglue/baseline-pdf"
)

// Attachment represents a file attachment in the PDF document It contains the
// name, description, mime type, data, and optionally creation/modification
// dates.
type Attachment struct {
	// Creation date of the attachment
	CreationDate time.Time
	// Modified date of the attachment
	ModDate time.Time
	// Name of the attachment (the visible name in the PDF)
	// This is the name of the file as it will be displayed in the PDF viewer
	// and is not necessarily the same as the original file name.
	Name string
	// Description of the attachment
	Description string
	// MimeType of the attachment
	MimeType string
	// Data of the attachment
	Data []byte
}

// AttachFile attaches a file to the document
func (d *PDFDocument) AttachFile(a Attachment) {
	d.Attachments = append(d.Attachments, a)
}

// embedFilespec serializes one attachment as an EmbeddedFile stream plus the
// /Filespec dictionary that references it, and returns the saved Filespec
// object. afRelationship sets /AFRelationship (e.g. "Alternative" for
// document attachments, "Supplement" for a MathML representation of a
// formula). The caller decides where to reference the returned Filespec —
// the document catalog /AF (document-level attachments) or a structure
// element's /AF (associated files, PDF 2.0 §14.13).
func (d *PDFDocument) embedFilespec(attachment Attachment, afRelationship string) (*pdf.Object, error) {
	pdfAttachment := d.PDFWriter.NewObject()
	pdfAttachment.Dictionary = pdf.Dict{
		"Type":    "/EmbeddedFile",
		"Length":  fmt.Sprintf("%d", len(attachment.Data)),
		"Subtype": pdf.Name(attachment.MimeType),
		"Params": pdf.Dict{
			"Size": fmt.Sprintf("%d", len(attachment.Data)),
		},
	}
	if !attachment.ModDate.IsZero() {
		pdfAttachment.Dictionary["Params"].(pdf.Dict)["ModDate"] = formatDate(attachment.ModDate.UTC())
	}
	pdfAttachment.SetCompression(9)
	pdfAttachment.Data.Write(attachment.Data)
	if err := pdfAttachment.Save(); err != nil {
		return nil, err
	}
	filespec := d.PDFWriter.NewObject()
	filespec.Dictionary = pdf.Dict{
		"Type":           "/Filespec",
		"AFRelationship": pdf.Name(afRelationship),
		"F":              pdf.String(attachment.Name),
		"UF":             pdf.String(attachment.Name),
		"EF": pdf.Dict{
			"F":  pdfAttachment.ObjectNumber.Ref(),
			"UF": pdfAttachment.ObjectNumber.Ref(),
		},
		"Desc": pdf.String(attachment.Description),
	}
	if err := filespec.Save(); err != nil {
		return nil, err
	}
	return filespec, nil
}
