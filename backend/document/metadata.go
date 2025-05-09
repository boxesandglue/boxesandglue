package document

import (
	"io"
	"time"

	"github.com/beevik/etree"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/google/uuid"
)

func (d *PDFDocument) getMetadata(w io.Writer) {
	var inner *etree.Document
	if d.AdditionalXMLMetadata != "" {
		inner = etree.NewDocument()
		if err := inner.ReadFromString("<dummy>" + d.AdditionalXMLMetadata + "</dummy>"); err != nil {
			bag.Logger.Error("error reading additional XML metadata", "error", err)
		}
	}
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
		desc.CreateElement("xmpMM:RenditionClass").SetText("default")
		desc.CreateElement("pdfaid:part").SetText("3")
		desc.CreateElement("pdfaid:conformance").SetText("B")
	case FormatPDFX4, FormatPDFX3:
		desc.CreateElement("xmpMM:RenditionClass").SetText("default")
		desc.CreateElement("xmpMM:VersionID").SetText("1")
		desc.CreateElement("pdf:Trapped").SetText("False")
		if d.Format == FormatPDFX3 {
			desc.CreateAttr("xmlns:pdfx", "http://ns.adobe.com/pdfx/1.3/")
			desc.CreateElement("pdfx:GTS_PDFXVersion").SetText("PDF/X-3:2002")
		} else if d.Format == FormatPDFX4 {
			desc.CreateAttr("xmlns:pdfxid", "http://www.npes.org/pdfx/ns/id/")
			desc.CreateElement("pdfxid:GTS_PDFXVersion").SetText("PDF/X-4")
		}
	}

	desc.CreateElement("xmpMM:DocumentID").SetText("uuid:" + docID)
	desc.CreateElement("xmpMM:InstanceID").SetText("uuid:" + instanceID)
	desc.CreateElement("xmp:CreateDate").SetText(formattedDate)
	desc.CreateElement("xmp:ModifyDate").SetText(formattedDate)
	desc.CreateElement("xmp:MetadataDate").SetText(formattedDate)
	desc.CreateElement("xmp:CreatorTool").SetText(d.Creator)
	desc.CreateElement("pdf:Producer").SetText(d.producer)
	if t := d.Title; t != "" {
		if d.Format == FormatPDFA3b {
			li := desc.CreateElement("dc:title").CreateElement("rdf:Alt").CreateElement("rdf:li")
			li.CreateAttr("xml:lang", "x-default")
			li.SetText(t)
		} else {
			desc.CreateElement("dc:title").SetText(t)
		}
	}
	if a := d.Author; a != "" {
		if d.Format == FormatPDFA3b {
			desc.CreateElement("dc:creator").CreateElement("rdf:Seq").CreateElement("rdf:li").SetText(a)
		} else {

			desc.CreateElement("dc:creator").SetText(a)
		}
	}
	if inner != nil {
		for _, v := range inner.Root().ChildElements() {
			rdf.AddChild(v.Copy())
		}
	}
	doc.Indent(2)
	_, _ = doc.WriteTo(w)
}
