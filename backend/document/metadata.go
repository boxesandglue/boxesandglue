package document

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var xmlescape = strings.NewReplacer("<", "&lt;", "&", "&amp;")

func (d *Document) getMetadata() string {
	var dateFormat = time.RFC3339
	pdfCreationdate := time.Now()
	isoformatted := pdfCreationdate.Format(dateFormat)
	docID := uuid.New()
	instanceID := uuid.New()

	str := `<?xpacket begin="%s" id="W5M0MpCehiHzreSzNTczkc9d"?>
	<x:xmpmeta xmlns:x="adobe:ns:meta/">
   <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
	 <rdf:Description rdf:about="" xmlns:xmpMM="http://ns.adobe.com/xap/1.0/mm/">
	   <xmpMM:DocumentID>uuid:%s</xmpMM:DocumentID>
	   <xmpMM:InstanceID>uuid:%s</xmpMM:InstanceID>
	 </rdf:Description>%s
	 <rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/">
		<xmp:CreateDate>%s</xmp:CreateDate>
		<xmp:ModifyDate>%s</xmp:ModifyDate>
		<xmp:MetadataDate>%s</xmp:MetadataDate>
		<xmp:CreatorTool>%s</xmp:CreatorTool>
	 </rdf:Description>
	 <rdf:Description rdf:about="" xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
	   <pdf:Producer>speedata Publisher</pdf:Producer>
	 </rdf:Description>
	 <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
	   <dc:title>
		 <rdf:Alt>
		   <rdf:li xml:lang="x-default">%s</rdf:li>
		 </rdf:Alt>
	   </dc:title>
	 </rdf:Description>
   </rdf:RDF>
 </x:xmpmeta>
<?xpacket end="r"?>`
	var pdfuaident string
	if d.RootStructureElement != nil {
		pdfuaident = `
<rdf:Description rdf:about="" xmlns:pdfuaid="http://www.aiim.org/pdfua/ns/id/">
	<pdfuaid:part>1</pdfuaid:part>
</rdf:Description>`
	}
	return fmt.Sprintf(str, "\xEF\xBB\xBF", docID, instanceID, pdfuaident, isoformatted, isoformatted, isoformatted, "speedata Publisher", xmlescape.Replace(d.Title))

}
