package document

import "time"

type Attachment struct {
	// Name of the attachment
	Name string
	// Description of the attachment
	Description string
	// MimeType of the attachment
	MimeType string
	// Data of the attachment
	Data []byte
	// Creation date of the attachment
	CreationDate time.Time
	// Modified date of the attachment
	ModDate time.Time
}

// AttachFile attaches a file to the document
func (d *PDFDocument) AttachFile(a Attachment) {
	d.Attachments = append(d.Attachments, a)
}
