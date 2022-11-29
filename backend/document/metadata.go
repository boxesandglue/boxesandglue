package document

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var xmlescape = strings.NewReplacer("<", "&lt;", "&", "&amp;")

func (d *PDFDocument) getMetadata() string {
	var dateFormat = time.RFC3339
	pdfCreationdate := time.Now()
	isoformatted := pdfCreationdate.Format(dateFormat)
	docID := uuid.New()
	instanceID := uuid.New()

	str := `<?xpacket begin="%[1]s" id="W5M0MpCehiHzreSzNTczkc9d"?>
	<x:xmpmeta xmlns:x="adobe:ns:meta/">
   <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
	 <rdf:Description rdf:about="" xmlns:xmpMM="http://ns.adobe.com/xap/1.0/mm/">
	   <xmpMM:DocumentID>uuid:%[2]s</xmpMM:DocumentID>
	   <xmpMM:InstanceID>uuid:%[3]s</xmpMM:InstanceID>
	 </rdf:Description>%[4]s
	 <rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/">
		<xmp:CreateDate>%[5]s</xmp:CreateDate>
		<xmp:ModifyDate>%[5]s</xmp:ModifyDate>
		<xmp:MetadataDate>%[5]s</xmp:MetadataDate>
		<xmp:CreatorTool>%[6]s</xmp:CreatorTool>
	 </rdf:Description>
	 <rdf:Description rdf:about="" xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
	   <pdf:Producer>%[7]s</pdf:Producer>%[10]s
	 </rdf:Description>
	 <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
	   <dc:title>%[8]s</dc:title>
	   <dc:creator>%[9]s</dc:creator>
	 </rdf:Description>
   </rdf:RDF>
 </x:xmpmeta>
<?xpacket end="r"?>`

	var pdfuaident string
	var keywords string
	if d.Keywords != "" {
		keywords = fmt.Sprintf(`
        <pdf:keywords>%s</pdf:keywords>`, xmlescape.Replace(d.Keywords))
	}
	if d.RootStructureElement != nil {
		pdfuaident = `
<rdf:Description rdf:about="" xmlns:pdfuaid="http://www.aiim.org/pdfua/ns/id/">
	<pdfuaid:part>1</pdfuaid:part>
</rdf:Description>`
	}
	return fmt.Sprintf(str,
		"\xEF\xBB\xBF",
		docID,
		instanceID,
		pdfuaident,
		isoformatted,
		xmlescape.Replace(d.Creator),
		xmlescape.Replace(d.producer),
		xmlescape.Replace(d.Title),
		xmlescape.Replace(d.Author),
		keywords,
	)

}
