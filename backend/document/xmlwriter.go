package document

import (
	"strings"
	"unicode/utf8"
)

// A Token is an interface type used to represent XML elements, character
// data, CDATA sections, XML comments, XML directives, and XML processing
// instructions.
type Token interface {
	// Parent() *Element
	// Index() int
	WriteTo(w strings.Builder)
	// dup(parent *Element) Token
	setParent(parent *Element)
	// setIndex(index int)
}

// spaceDecompose breaks a namespace:tag identifier at the ':'
// and returns the two parts.
func spaceDecompose(str string) (space, key string) {
	colon := strings.IndexByte(str, ':')
	if colon == -1 {
		return "", str
	}
	return str[:colon], str[colon+1:]
}

// CreateElement creates a new element with the specified tag (i.e., name) and
// adds it as the last child of element 'e'. The tag may include a prefix
// followed by a colon.
func (e *Element) CreateElement(tag string) *Element {
	space, stag := spaceDecompose(tag)
	return newElement(space, stag, e)
}

// FullTag returns the element e's complete tag, including namespace prefix if
// present.
func (e *Element) FullTag() string {
	if e.Space == "" {
		return e.Tag
	}
	return e.Space + ":" + e.Tag
}

// WriteTo serializes the element to the writer w.
func (e *Element) WriteTo(w strings.Builder) {
	w.WriteByte('<')
	w.WriteString(e.FullTag())
	for _, a := range e.Attr {
		w.WriteByte(' ')
		a.WriteTo(w)
	}
	if len(e.Child) > 0 {
		w.WriteByte('>')
		for _, c := range e.Child {
			c.WriteTo(w)
		}
		w.Write([]byte{'<', '/'})
		w.WriteString(e.FullTag())
		w.WriteByte('>')
	} else {
		w.Write([]byte{'/', '>'})
	}
}

// newElement is a helper function that creates an element and binds it to
// a parent element if possible.
func newElement(space, tag string, parent *Element) *Element {
	e := &Element{
		Space:  space,
		Tag:    tag,
		Attr:   make([]Attr, 0),
		Child:  make([]Token, 0),
		parent: parent,
	}
	if parent != nil {
		parent.addChild(e)
	}
	return e
}

// setParent replaces this element token's parent.
func (e *Element) setParent(parent *Element) {
	e.parent = parent
}

// addChild adds a child token to the element e.
func (e *Element) addChild(t Token) {
	t.setParent(e)
	e.Child = append(e.Child, t)
}

type Element struct {
	Space, Tag string   // namespace prefix and tag
	Attr       []Attr   // key-value attribute pairs
	Child      []Token  // child tokens (elements, comments, etc.)
	parent     *Element // parent element
}

type escapeMode byte

const (
	escapeNormal escapeMode = iota
	escapeCanonicalText
	escapeCanonicalAttr
)

func isInCharacterRange(r rune) bool {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}

// escapeString writes an escaped version of a string to the writer.
func escapeString(w strings.Builder, s string, m escapeMode) {
	var esc []byte
	last := 0
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		i += width
		switch r {
		case '&':
			esc = []byte("&amp;")
		case '<':
			esc = []byte("&lt;")
		case '>':
			if m == escapeCanonicalAttr {
				continue
			}
			esc = []byte("&gt;")
		case '\'':
			if m != escapeNormal {
				continue
			}
			esc = []byte("&apos;")
		case '"':
			if m == escapeCanonicalText {
				continue
			}
			esc = []byte("&quot;")
		case '\t':
			if m != escapeCanonicalAttr {
				continue
			}
			esc = []byte("&#x9;")
		case '\n':
			if m != escapeCanonicalAttr {
				continue
			}
			esc = []byte("&#xA;")
		case '\r':
			if m == escapeNormal {
				continue
			}
			esc = []byte("&#xD;")
		default:
			if !isInCharacterRange(r) || (r == 0xFFFD && width == 1) {
				esc = []byte("\uFFFD")
				break
			}
			continue
		}
		w.WriteString(s[last : i-width])
		w.Write(esc)
		last = i
	}
	w.WriteString(s[last:])
}

// FullKey returns this attribute's complete key, including namespace prefix
// if present.
func (a *Attr) FullKey() string {
	if a.Space == "" {
		return a.Key
	}
	return a.Space + ":" + a.Key
}

// WriteTo serializes the attribute to the writer.
func (a *Attr) WriteTo(w strings.Builder) {
	w.WriteString(a.FullKey())
	w.WriteString(`="`)
	escapeString(w, a.Value, escapeNormal)
	w.WriteByte('"')
}

// An Attr represents a key-value attribute within an XML element.
type Attr struct {
	Space, Key string   // The attribute's namespace prefix and key
	Value      string   // The attribute value string
	element    *Element // element containing the attribute
}

type XMLDocument struct {
	Element
}
