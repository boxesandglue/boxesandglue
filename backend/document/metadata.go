package document

import (
	"io"
	"sort"
	"time"

	"github.com/google/uuid"
)

// XMPExtensionProperty describes a single property within an XMP extension
// schema. Category is typically "external" or "internal", ValueType is the
// XMP type name (e.g. "Text", "Integer").
type XMPExtensionProperty struct {
	Name        string
	ValueType   string
	Category    string
	Description string
}

// XMPExtension declares an XMP extension schema (pdfaExtension:schemas) and
// the property values to write under that namespace. This is needed for PDF/A
// compliance when using custom metadata namespaces (e.g. ZUGFeRD).
//
// Schema, NamespaceURI, and Prefix describe the extension schema itself.
// Properties declares the property definitions (for the schema block).
// Values holds the actual property values to emit (prefix:Name = value).
type XMPExtension struct {
	Schema       string                 // human-readable schema name
	NamespaceURI string                 // namespace URI
	Prefix       string                 // XML prefix
	Properties   []XMPExtensionProperty // property declarations
	Values       map[string]string      // property name → value
}

// AddXMPExtension registers an XMP extension schema on the document. The
// extension will be rendered into the XMP metadata stream when the PDF is
// finalized.
func (d *PDFDocument) AddXMPExtension(ext XMPExtension) {
	d.xmpExtensions = append(d.xmpExtensions, ext)
}

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

	x := newXMLWriter(w)
	x.procInst("xpacket", `begin="ï»¿" id="W5M0MpCehiHzreSzNTczkc9d"`)

	x.start("x:xmpmeta", "xmlns:x", "adobe:ns:meta/")
	x.start("rdf:RDF", "xmlns:rdf", "http://www.w3.org/1999/02/22-rdf-syntax-ns#")

	// Main description block with standard namespaces.
	descAttrs := []string{
		"rdf:about", "",
		"xmlns:xmpMM", "http://ns.adobe.com/xap/1.0/mm/",
		"xmlns:xmp", "http://ns.adobe.com/xap/1.0/",
		"xmlns:pdf", "http://ns.adobe.com/pdf/1.3/",
		"xmlns:dc", "http://purl.org/dc/elements/1.1/",
		"xmlns:pdfuaid", "http://www.aiim.org/pdfua/ns/id/",
		"xmlns:pdfaid", "http://www.aiim.org/pdfa/ns/id/",
	}
	switch d.Format {
	case FormatPDFX3:
		descAttrs = append(descAttrs, "xmlns:pdfx", "http://ns.adobe.com/pdfx/1.3/")
	case FormatPDFX4:
		descAttrs = append(descAttrs, "xmlns:pdfxid", "http://www.npes.org/pdfx/ns/id/")
	}
	x.start("rdf:Description", descAttrs...)

	switch d.Format {
	case FormatPDFA3b:
		x.textElement("xmpMM:RenditionClass", "default")
		x.textElement("pdfaid:part", "3")
		x.textElement("pdfaid:conformance", "B")
	case FormatPDFX4, FormatPDFX3:
		x.textElement("xmpMM:RenditionClass", "default")
		x.textElement("xmpMM:VersionID", "1")
		x.textElement("pdf:Trapped", "False")
		switch d.Format {
		case FormatPDFX3:
			x.textElement("pdfx:GTS_PDFXVersion", "PDF/X-3:2002")
		case FormatPDFX4:
			x.textElement("pdfxid:GTS_PDFXVersion", "PDF/X-4")
		}
	case FormatPDFUA:
		x.textElement("pdfuaid:part", "1")
	}

	x.textElement("xmpMM:DocumentID", "uuid:"+docID)
	x.textElement("xmpMM:InstanceID", "uuid:"+instanceID)
	x.textElement("xmp:CreateDate", formattedDate)
	x.textElement("xmp:ModifyDate", formattedDate)
	x.textElement("xmp:MetadataDate", formattedDate)
	x.textElement("xmp:CreatorTool", d.Creator)
	x.textElement("pdf:Producer", d.producer)

	if t := d.Title; t != "" {
		switch d.Format {
		case FormatPDFA3b, FormatPDFUA:
			x.start("dc:title")
			x.start("rdf:Alt")
			x.start("rdf:li", "xml:lang", "x-default")
			x.text(t)
			x.end("rdf:li")
			x.end("rdf:Alt")
			x.end("dc:title")
		default:
			x.textElement("dc:title", t)
		}
	}
	if a := d.Author; a != "" {
		if d.Format == FormatPDFA3b {
			x.start("dc:creator")
			x.start("rdf:Seq")
			x.start("rdf:li")
			x.text(a)
			x.end("rdf:li")
			x.end("rdf:Seq")
			x.end("dc:creator")
		} else {
			x.textElement("dc:creator", a)
		}
	}

	x.end("rdf:Description")

	// Render structured XMP extensions.
	if len(d.xmpExtensions) > 0 {
		// Schema declarations block.
		x.start("rdf:Description",
			"rdf:about", "",
			"xmlns:pdfaExtension", "http://www.aiim.org/pdfa/ns/extension/",
			"xmlns:pdfaSchema", "http://www.aiim.org/pdfa/ns/schema#",
			"xmlns:pdfaProperty", "http://www.aiim.org/pdfa/ns/property#",
		)
		x.start("pdfaExtension:schemas")
		x.start("rdf:Bag")

		for _, ext := range d.xmpExtensions {
			x.start("rdf:li", "rdf:parseType", "Resource")
			x.textElement("pdfaSchema:schema", ext.Schema)
			x.textElement("pdfaSchema:namespaceURI", ext.NamespaceURI)
			x.textElement("pdfaSchema:prefix", ext.Prefix)

			if len(ext.Properties) > 0 {
				x.start("pdfaSchema:property")
				x.start("rdf:Seq")
				for _, p := range ext.Properties {
					x.start("rdf:li", "rdf:parseType", "Resource")
					x.textElement("pdfaProperty:name", p.Name)
					x.textElement("pdfaProperty:valueType", p.ValueType)
					x.textElement("pdfaProperty:category", p.Category)
					x.textElement("pdfaProperty:description", p.Description)
					x.end("rdf:li")
				}
				x.end("rdf:Seq")
				x.end("pdfaSchema:property")
			}

			x.end("rdf:li")
		}

		x.end("rdf:Bag")
		x.end("pdfaExtension:schemas")
		x.end("rdf:Description")

		// Value descriptions — one rdf:Description per extension.
		for _, ext := range d.xmpExtensions {
			if len(ext.Values) == 0 {
				continue
			}
			attrs := []string{
				"xmlns:" + ext.Prefix, ext.NamespaceURI,
				"rdf:about", "",
			}
			keys := make([]string, 0, len(ext.Values))
			for k := range ext.Values {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				attrs = append(attrs, ext.Prefix+":"+k, ext.Values[k])
			}
			x.empty("rdf:Description", attrs...)
		}
	}

	x.end("rdf:RDF")
	x.end("x:xmpmeta")
}
