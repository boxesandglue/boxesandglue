package document

import (
	"fmt"
	"io"
	"strings"
)

// xmlWriter is a minimal XML builder for generating XMP metadata streams.
// It supports elements, attributes, text content, processing instructions,
// and automatic indentation.
type xmlWriter struct {
	w     io.Writer
	depth int
}

func newXMLWriter(w io.Writer) *xmlWriter {
	return &xmlWriter{w: w}
}

// procInst writes a processing instruction: <?target inst?>
func (x *xmlWriter) procInst(target, inst string) {
	fmt.Fprintf(x.w, "<?%s %s?>\n", target, inst)
}

// start writes an opening tag with optional attributes. Attributes are
// given as key/value pairs: start("rdf:Description", "rdf:about", "").
func (x *xmlWriter) start(name string, attrs ...string) {
	x.writeIndent()
	fmt.Fprintf(x.w, "<%s", name)
	for i := 0; i+1 < len(attrs); i += 2 {
		fmt.Fprintf(x.w, " %s=%q", attrs[i], attrs[i+1])
	}
	fmt.Fprint(x.w, ">\n")
	x.depth++
}

// end writes a closing tag.
func (x *xmlWriter) end(name string) {
	x.depth--
	x.writeIndent()
	fmt.Fprintf(x.w, "</%s>\n", name)
}

// empty writes a self-closing tag with optional attributes.
func (x *xmlWriter) empty(name string, attrs ...string) {
	x.writeIndent()
	fmt.Fprintf(x.w, "<%s", name)
	for i := 0; i+1 < len(attrs); i += 2 {
		fmt.Fprintf(x.w, " %s=%q", attrs[i], attrs[i+1])
	}
	fmt.Fprint(x.w, "/>\n")
}

// text writes raw text content (XML-escaped) at the current position,
// without any surrounding element or indentation.
func (x *xmlWriter) text(s string) {
	fmt.Fprint(x.w, xmlEscape(s))
}

// textElement writes an element with text content on a single line:
// <name>text</name>
func (x *xmlWriter) textElement(name, text string) {
	x.writeIndent()
	fmt.Fprintf(x.w, "<%s>%s</%s>\n", name, xmlEscape(text), name)
}

func (x *xmlWriter) writeIndent() {
	fmt.Fprint(x.w, strings.Repeat("  ", x.depth))
}

// xmlEscape escapes the five XML special characters.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"'", "&apos;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}
