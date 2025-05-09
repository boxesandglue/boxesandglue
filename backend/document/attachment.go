package document

import "time"

// Attachment represents a file attachment in the PDF document It contains the
// name, description, mime type, data, and optionally creation/modification
// dates.
type Attachment struct {
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
	// Creation date of the attachment
	CreationDate time.Time
	// Modified date of the attachment
	ModDate time.Time
}

// AttachFile attaches a file to the document
func (d *PDFDocument) AttachFile(a Attachment) {
	d.Attachments = append(d.Attachments, a)
}
