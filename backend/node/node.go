package node

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

var nextID atomic.Int64

// Type is the type of node.
type Type int

const (
	// TypeUnknown is a node which type is unknown.
	TypeUnknown Type = iota
	// TypeDisc is a Disc node.
	TypeDisc
	// TypeGlue is a Glue node.
	TypeGlue
	// TypeGlyph is a Glyph node.
	TypeGlyph
	// TypeHList is a HList node.
	TypeHList
	// TypeImage is a Image node.
	TypeImage
	// TypeKern is a Kern node.
	TypeKern
	// TypeLang is a Lang node.
	TypeLang
	// TypePenalty is a Penalty node.
	TypePenalty
	// TypeRule is a Rule node.
	TypeRule
	// TypeStartStop marks the beginning and end of a something interesting.
	TypeStartStop
	// TypeVList is a VList node.
	TypeVList
	// TypeHardBreak is a forced line break (HTML <br>, source "\n").
	TypeHardBreak
)

// typeMetadata is the single source of truth for a node Type's
// human-readable strings. Index by Type constant. Adding a new node type
// requires exactly one entry here plus a slab in arena.go and the type
// constant above.
var typeMetadata = [...]struct {
	debug   string // mixed case, used by Type.String() for logs
	element string // lowercase, used by Name() for debug XML output
}{
	TypeUnknown:   {"Unknown", "unknown"},
	TypeDisc:      {"Disc", "disc"},
	TypeGlue:      {"Glue", "glue"},
	TypeGlyph:     {"Glyph", "glyph"},
	TypeHList:     {"HList", "hlist"},
	TypeImage:     {"Image", "image"},
	TypeKern:      {"Kern", "kern"},
	TypeLang:      {"Lang", "lang"},
	TypePenalty:   {"Penalty", "penalty"},
	TypeRule:      {"Rule", "rule"},
	TypeStartStop: {"StartStop", "startstop"},
	TypeVList:     {"Vlist", "vlist"},
	TypeHardBreak: {"HardBreak", "hardbreak"},
}

// String returns the mixed-case name used in log output.
func (t Type) String() string {
	if int(t) >= 0 && int(t) < len(typeMetadata) {
		return typeMetadata[t].debug
	}
	return "something else"
}

// VerticalAlignment sets the alignment in horizontal lists (hlist). The default
// alignment is VAlignBaseline which means that all items in the hlist have the
// same base line.
type VerticalAlignment uint

const (
	// VAlignBaseline is the default alignment in hlists which has all items
	// aligned at the base line.
	VAlignBaseline VerticalAlignment = 0
	// VAlignTop has all items in a hlist hanging down from the top like
	// stalactites in a cave.
	VAlignTop = 1
)

// H is a shortcut for map[string]any
type H map[string]any

// Node represents any kind of node
type Node interface {
	Next() Node
	Prev() Node
	SetNext(Node)
	SetPrev(Node)
	GetID() int
	Type() Type
	Name() string
	Copy() Node
	// Sizes returns the natural width, height and depth of the node in the
	// given progression direction. Nodes that do not contribute to box
	// geometry (Disc, Lang, StartStop, HardBreak) return zeros.
	Sizes(dir Direction) (w, h, d bag.ScaledPoint)
	// GetAttribute reads a per-node attribute.
	GetAttribute(attr string) (any, bool)
	// SetAttribute stores a per-node attribute.
	SetAttribute(attr string, val any)
	// DebugAttributes returns the typed key/value pairs to emit for this
	// node in debug XML output, together with the user-defined attribute
	// map. The default implementation on basenode returns just {id} plus
	// b.Attributes; geometry-bearing types override.
	DebugAttributes() ([]kv, H)
	// BidiLevel returns the UAX#9 embedding level of this node. 0 = LTR
	// (default). Odd levels are RTL.
	BidiLevel() uint8
	// SetBidiLevel records the UAX#9 embedding level of this node. The level
	// is used by the post-linebreak reorder to put runs into visual order.
	SetBidiLevel(uint8)
}

func showRecentNodes(n Node, i int) string {
	ret := []string{}
	c := 0
	for e := n; e != nil; e = e.Prev() {
		switch t := e.(type) {
		case *Glue:
			ret = append(ret, " ")
		case *Glyph:
			ret = append(ret, t.Components)
		case *Disc:
			ret = append(ret, "|")
		case *Penalty:
			ret = append(ret, "•")
		case *Kern:
			c--
			// ignore
		default:
			c--
			fmt.Printf("**%T\n", t)
		}
		c++
		if c >= i {
			break
		}
	}

	j := 0
	input := strings.Join(ret, "")
	rune := make([]rune, len(input))
	for _, r := range input {
		rune[j] = r
		j++
	}
	rune = rune[0:j]
	// Reverse
	for i := 0; i < j/2; i++ {
		rune[i], rune[j-1-i] = rune[j-1-i], rune[i]
	}
	return string(rune)
}

// String returns a string representation of the node n and the previous and
// next node.
func String(n Node) string {
	var nx, pr, extrainfo string
	if next := n.Next(); next != nil {
		nx = fmt.Sprintf("%s %d", next.Name(), next.GetID())
	} else {
		nx = "-"
	}
	if prev := n.Prev(); prev != nil {
		pr = fmt.Sprintf("%s %d", prev.Name(), prev.GetID())
	} else {
		pr = "-"
	}
	switch t := n.(type) {
	case *Glue:
		extrainfo = fmt.Sprintf(": %spt plus %s", t.Width, t.Stretch)
		if t.Leader != nil {
			extrainfo += " (leader)"
		}
	case *Glyph:
		var fontname string
		if t.Font != nil && t.Font.Face != nil {
			fontname = fmt.Sprintf("font: %s", t.Font.Face.InternalName())
		}
		extrainfo = fmt.Sprintf(": %s (%s)", t.Components, fontname)
	case *Kern:
		extrainfo = t.Kern.String()
	}
	return fmt.Sprintf(" %12s <- %-10s %4d -> %12s%s", pr, n.Name(), n.GetID(), nx, extrainfo)
}

// StringValue returns a short string representation of the node list starting at n.
func StringValue(n Node) string {
	var sb strings.Builder
	for e := n; e != nil; e = e.Next() {
		switch e.Type() {
		case TypeGlyph:
			sb.WriteString(e.(*Glyph).Components)
		default:
			sb.WriteString(".")
		}
	}
	return sb.String()
}

type basenode struct {
	next       Node
	prev       Node
	Attributes H
	ID         int
	typ        Type
	bidiLevel  uint8
}

// BidiLevel returns the UAX#9 embedding level of this node. 0 means LTR
// (default). Odd levels are RTL.
func (b *basenode) BidiLevel() uint8 {
	return b.bidiLevel
}

// SetBidiLevel sets the UAX#9 embedding level of this node.
func (b *basenode) SetBidiLevel(level uint8) {
	b.bidiLevel = level
}

// Next returns the following node or nil if no such node exists.
func (b *basenode) Next() Node { return b.next }

// Prev returns the preceding node or nil if no such node exists.
func (b *basenode) Prev() Node { return b.prev }

// SetNext sets the following node.
func (b *basenode) SetNext(n Node) { b.next = n }

// SetPrev sets the preceding node.
func (b *basenode) SetPrev(n Node) { b.prev = n }

// GetID returns the node id.
func (b *basenode) GetID() int { return b.ID }

// Sizes returns the natural width, height and depth of the node when laid
// out in the given progression direction. The default implementation
// returns zero on all three axes; container or geometry-bearing nodes
// override it.
func (b *basenode) Sizes(Direction) (w, h, d bag.ScaledPoint) { return }

// GetAttribute returns the value stored under attr and true, or nil and
// false if no such attribute is set.
func (b *basenode) GetAttribute(attr string) (any, bool) {
	if b.Attributes == nil {
		return nil, false
	}
	v, ok := b.Attributes[attr]
	return v, ok
}

// SetAttribute stores val under attr, allocating the attribute map on
// first use.
func (b *basenode) SetAttribute(attr string, val any) {
	if b.Attributes == nil {
		b.Attributes = H{}
	}
	b.Attributes[attr] = val
}

// Type returns the type code stamped on the node by its constructor.
func (b *basenode) Type() Type { return b.typ }

// Name returns the lowercase element name used for debug XML output.
func (b *basenode) Name() string {
	if int(b.typ) >= 0 && int(b.typ) < len(typeMetadata) {
		return typeMetadata[b.typ].element
	}
	return "unknown"
}

// DebugAttributes returns the minimal ID-only attribute set used by nodes
// without geometry (Disc, HardBreak). Geometry-bearing types override.
func (b *basenode) DebugAttributes() ([]kv, H) {
	return []kv{{key: "id", value: b.ID}}, b.Attributes
}

func newID() int {
	return int(nextID.Add(1))
}
