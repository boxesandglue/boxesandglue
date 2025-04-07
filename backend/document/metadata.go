package document

import (
	"io"
	"time"

	"github.com/beevik/etree"
	"github.com/google/uuid"
)

func (d *PDFDocument) getMetadata(w io.Writer) {

	var docID, instanceID string
	formattedDate := d.CreationDate.Format(time.RFC3339)
	if d.SuppressInfo {
		docID = "fbb12364-c841-4d5e-b1b0-f53bd6e22649"
		instanceID = "fbb12364-c841-4d5e-b1b0-f53bd6e22649"
	} else {
		docID = uuid.New().String()
		instanceID = uuid.New().String()
	}
	doc := etree.NewDocument()
	doc.CreateProcInst("xpacket", `begin="ï»¿" id="W5M0MpCehiHzreSzNTczkc9d"`)

	meta := doc.CreateElement("x:xmpmeta")
	meta.CreateAttr("xmlns:x", "adobe:ns:meta/")

	rdf := meta.CreateElement("rdf:RDF")
	rdf.CreateAttr("xmlns:rdf", "http://www.w3.org/1999/02/22-rdf-syntax-ns#")

	desc := rdf.CreateElement("rdf:Description")
	desc.CreateAttr("rdf:about", "")
	desc.CreateAttr("xmlns:xmpMM", "http://ns.adobe.com/xap/1.0/mm/")
	desc.CreateAttr("xmlns:xmp", "http://ns.adobe.com/xap/1.0/")
	desc.CreateAttr("xmlns:pdf", "http://ns.adobe.com/pdf/1.3/")
	desc.CreateAttr("xmlns:dc", "http://purl.org/dc/elements/1.1/")
	desc.CreateAttr("xmlns:pdfuaid", "http://www.aiim.org/pdfua/ns/id/")
	desc.CreateAttr("xmlns:pdfaid", "http://www.aiim.org/pdfa/ns/id/")
	switch d.Format {
	case FormatPDFA3b:
		desc.CreateElement("pdfaid:part").SetText("3")
		desc.CreateElement("pdfaid:conformance").SetText("B")
	}

	desc.CreateElement("xmpMM:DocumentID").SetText("uuid:" + docID)
	desc.CreateElement("xmpMM:InstanceID").SetText("uuid:" + instanceID)
	desc.CreateElement("xmp:CreateDate").SetText(formattedDate)
	desc.CreateElement("xmp:ModifyDate").SetText(formattedDate)
	desc.CreateElement("xmp:MetadataDate").SetText(formattedDate)
	desc.CreateElement("xmp:CreatorTool").SetText(d.Creator)
	desc.CreateElement("pdf:Producer").SetText(d.producer)
	if t := d.Title; t != "" {
		desc.CreateElement("dc:title").SetText(t)
	}
	if a := d.Author; a != "" {
		desc.CreateElement("dc:creator").SetText(a)
	}

	doc.Indent(2)
	_, _ = doc.WriteTo(w)
}
