package node

import (
	"fmt"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/image"
	"github.com/speedata/boxesandglue/backend/lang"
)

var (
	ids chan int
)

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
)

func (t Type) String() string {
	switch t {
	case TypeUnknown:
		return "Unknown"
	case TypeDisc:
		return "Disc"
	case TypeGlue:
		return "Glue"
	case TypeGlyph:
		return "Glyph"
	case TypeHList:
		return "HList"
	case TypeImage:
		return "Image"
	case TypeKern:
		return "Kern"
	case TypeLang:
		return "Lang"
	case TypePenalty:
		return "Penalty"
	case TypeRule:
		return "Rule"
	case TypeStartStop:
		return "StartStop"
	case TypeVList:
		return "Vlist"
	default:
		return "something else"
	}
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

type basenode struct {
	next       Node
	prev       Node
	ID         int
	Attributes H
}

func genIntegerSequence(ids chan int) {
	i := int(0)
	for {
		ids <- i
		i++
	}
}

func init() {
	ids = make(chan int)
	go genIntegerSequence(ids)
}

// IsNode returns true if the argument is a Node.
func IsNode(arg any) bool {
	switch arg.(type) {
	case *Disc, *Glyph, *Glue, *Image, *HList, *Kern, *Lang, *StartStop, *VList:
		return true
	}
	return false
}

// A Disc represents a hyphenation point. Currently only the Penalty field is
// used.
type Disc struct {
	basenode
	Pre     Node
	Post    Node
	Replace Node
	Penalty int // Added to the hyphen penalty
}

func (d *Disc) String() string {
	return String(d)
}

// NewDisc creates an initialized Disc node
func NewDisc() *Disc {
	n := &Disc{}
	n.ID = <-ids
	return n
}

// Next returns the following node or nil if no such node exists.
func (d *Disc) Next() Node {
	return d.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (d *Disc) Prev() Node {
	return d.prev
}

// SetNext sets the following node.
func (d *Disc) SetNext(n Node) {
	d.next = n
}

// SetPrev sets the preceding node.
func (d *Disc) SetPrev(n Node) {
	d.prev = n
}

// GetID returns the node id
func (d *Disc) GetID() int {
	return d.ID
}

// Name returns the name of the node
func (d *Disc) Name() string {
	return "disc"
}

// Type returns the type of the node
func (d *Disc) Type() Type {
	return TypeDisc
}

// Copy creates a deep copy of the node.
func (d *Disc) Copy() Node {
	n := NewDisc()
	n.Pre = CopyList(d.Pre)
	n.Post = CopyList(d.Post)
	n.Replace = CopyList(d.Replace)
	n.Penalty = d.Penalty
	return n
}

// NewDiscWithContents creates an initialized Disc node with the given contents
func NewDiscWithContents(n *Disc) *Disc {
	n.ID = <-ids
	return n
}

// IsDisc returns the value of the element and true, if the element is a Disc
// node.
func IsDisc(elt Node) (*Disc, bool) {
	Disc, ok := elt.(*Disc)
	return Disc, ok
}

// Glyph nodes represents a single visible entity such as a letter or a
// ligature.
type Glyph struct {
	basenode
	Font       *font.Font
	Codepoint  int    // The font specific glyph id
	Components string // A codepoint can contain more than one rune, for example a fi ligature contains f + i
	Width      bag.ScaledPoint
	Height     bag.ScaledPoint
	Depth      bag.ScaledPoint
	XOffset    bag.ScaledPoint
	YOffset    bag.ScaledPoint
	Hyphenate  bool
}

func (g *Glyph) String() string {
	return String(g)
}

// Next returns the following node or nil if no such node exists.
func (g *Glyph) Next() Node {
	return g.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (g *Glyph) Prev() Node {
	return g.prev
}

// SetNext sets the following node.
func (g *Glyph) SetNext(n Node) {
	g.next = n
}

// SetPrev sets the preceding node.
func (g *Glyph) SetPrev(n Node) {
	g.prev = n
}

// GetID returns the node id
func (g *Glyph) GetID() int {
	return g.ID
}

// Name returns the name of the node
func (g *Glyph) Name() string {
	return "glyph"
}

// Type returns the type of the node
func (g *Glyph) Type() Type {
	return TypeGlyph
}

// Copy creates a deep copy of the node.
func (g *Glyph) Copy() Node {
	n := NewGlyph()
	n.Font = g.Font
	n.Codepoint = g.Codepoint
	n.Components = g.Components
	n.Width = g.Width
	n.Height = g.Height
	n.Depth = g.Depth
	n.Hyphenate = g.Hyphenate
	return n
}

// NewGlyph returns an initialized Glyph
func NewGlyph() *Glyph {
	n := &Glyph{}
	n.ID = <-ids
	return n
}

// IsGlyph returns the value of the element and true, if the element is a Glyph
// node.
func IsGlyph(elt Node) (*Glyph, bool) {
	n, ok := elt.(*Glyph)
	return n, ok
}

// GlueOrder represents the stretch and shrink priority.
type GlueOrder int

const (
	// StretchNormal means no stretching
	StretchNormal GlueOrder = iota
	// StretchFil is the first order infinity
	StretchFil
	// StretchFill is the second order infinity
	StretchFill
	// StretchFilll is the third order infinity
	StretchFilll
)

// GlueSubtype is set wherever the glue comes from.
type GlueSubtype int

const (
	// GlueDefault when no subtype is set
	GlueDefault GlueSubtype = iota
	// GlueLineStart is inserted left of the hlist during the line breaking
	GlueLineStart
	// GlueLineEnd is added at the end of each line in a paragraph so that copy
	// and paste works in PDF.
	GlueLineEnd
)

// A Glue node has the value of a shrinking and stretching space
type Glue struct {
	basenode
	Subtype      GlueSubtype
	Width        bag.ScaledPoint // The natural width of the glue.
	Stretch      bag.ScaledPoint // The stretchability of the glue, where width plus stretch = maximum width.
	Shrink       bag.ScaledPoint // The shrinkability of the glue, where width minus shrink = minimum width.
	StretchOrder GlueOrder       // The order of infinity of stretching.
	ShrinkOrder  GlueOrder       // The order of infinity of shrinking.
}

func (g *Glue) String() string {
	return String(g)
}

// Next returns the following node or nil if no such node exists.
func (g *Glue) Next() Node {
	return g.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (g *Glue) Prev() Node {
	return g.prev
}

// SetNext sets the following node.
func (g *Glue) SetNext(n Node) {
	g.next = n
}

// SetPrev sets the preceding node.
func (g *Glue) SetPrev(n Node) {
	g.prev = n
}

// GetID returns the node id
func (g *Glue) GetID() int {
	return g.ID
}

// Name returns the name of the node
func (g *Glue) Name() string {
	return "glue"
}

// Type returns the type of the node
func (g *Glue) Type() Type {
	return TypeGlue
}

// Copy creates a deep copy of the node.
func (g *Glue) Copy() Node {
	n := NewGlue()
	n.Width = g.Width
	n.Stretch = g.Stretch
	n.Shrink = g.Shrink
	n.StretchOrder = g.StretchOrder
	n.ShrinkOrder = g.ShrinkOrder
	return n
}

// NewGlue creates an initialized Glue node
func NewGlue() *Glue {
	n := &Glue{}
	n.ID = <-ids
	return n
}

// IsGlue returns the value of the element and true, if the element is a Glue
// node.
func IsGlue(elt Node) (*Glue, bool) {
	n, ok := elt.(*Glue)
	return n, ok
}

// A HList is a container for a list which items are placed horizontally next to
// each other. The most convenient way to create a hlist is using node.HPack.
// The width, height, depth, badness and the glue settings are calculated when
// using node.HPack.
type HList struct {
	Width     bag.ScaledPoint
	Height    bag.ScaledPoint
	Depth     bag.ScaledPoint
	Badness   int
	GlueSet   float64         // The ratio of the glue. Positive means stretching, negative shrinking.
	GlueSign  uint8           // 0 = normal, 1 = stretching, 2 = shrinking
	GlueOrder GlueOrder       // The level of infinity
	Shift     bag.ScaledPoint // The displacement perpendicular to the progressing direction. Not used.
	List      Node            // The list itself.
	VAlign    VerticalAlignment
	basenode
}

func (h *HList) String() string {
	return String(h)
}

// Next returns the following node or nil if no such node exists.
func (h *HList) Next() Node {
	return h.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (h *HList) Prev() Node {
	return h.prev
}

// SetNext sets the following node.
func (h *HList) SetNext(n Node) {
	h.next = n
}

// SetPrev sets the preceding node.
func (h *HList) SetPrev(n Node) {
	h.prev = n
}

// GetID returns the node id
func (h *HList) GetID() int {
	return h.ID
}

// Name returns the name of the node
func (h *HList) Name() string {
	return "hlist"
}

// Type returns the type of the node
func (h *HList) Type() Type {
	return TypeHList
}

// Copy creates a deep copy of the node.
func (h *HList) Copy() Node {
	n := NewHList()
	n.Width = h.Width
	n.Height = h.Height
	n.Depth = h.Depth
	n.GlueSet = h.GlueSet
	n.GlueSign = h.GlueSign
	n.Shift = h.Shift
	n.List = CopyList(h.List)
	return n
}

// NewHList creates an initialized HList node
func NewHList() *HList {
	n := &HList{}
	n.ID = <-ids
	return n
}

// IsHList returns the value of the element and true, if the element is a HList
// node.
func IsHList(elt Node) (*HList, bool) {
	hlist, ok := elt.(*HList)
	return hlist, ok
}

// A Kern is a small space between glyphs.
type Kern struct {
	// The displacement in progression direction.
	Kern bag.ScaledPoint
	basenode
}

func (k *Kern) String() string {
	return String(k)
}

// Next returns the following node or nil if no such node exists.
func (k *Kern) Next() Node {
	return k.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (k *Kern) Prev() Node {
	return k.prev
}

// SetNext sets the following node.
func (k *Kern) SetNext(n Node) {
	k.next = n
}

// SetPrev sets the preceding node.
func (k *Kern) SetPrev(n Node) {
	k.prev = n
}

// GetID returns the node id
func (k *Kern) GetID() int {
	return k.ID
}

// Name returns the name of the node
func (k *Kern) Name() string {
	return "kern"
}

// Type returns the type of the node
func (k *Kern) Type() Type {
	return TypeKern
}

// Copy creates a deep copy of the node.
func (k *Kern) Copy() Node {
	n := NewKern()
	n.Kern = k.Kern
	return n
}

// NewKern creates an initialized Kern node
func NewKern() *Kern {
	n := &Kern{}
	n.ID = <-ids
	return n
}

// IsKern returns the value of the element and true, if the element is a Kern
// node.
func IsKern(elt Node) (*Kern, bool) {
	n, ok := elt.(*Kern)
	return n, ok
}

// A Lang is a node that sets the current language.
type Lang struct {
	basenode
	Lang *lang.Lang // The language setting  for the following nodes.
}

func (l *Lang) String() string {
	return "lang: " + l.Lang.Name
}

// Next returns the following node or nil if no such node exists.
func (l *Lang) Next() Node {
	return l.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (l *Lang) Prev() Node {
	return l.prev
}

// SetNext sets the following node.
func (l *Lang) SetNext(n Node) {
	l.next = n
}

// SetPrev sets the preceding node.
func (l *Lang) SetPrev(n Node) {
	l.prev = n
}

// GetID returns the node id
func (l *Lang) GetID() int {
	return l.ID
}

// Name returns the name of the node
func (l *Lang) Name() string {
	return "lang"
}

// Copy creates a deep copy of the node.
func (l *Lang) Copy() Node {
	n := NewLang()
	n.Lang = l.Lang
	return n
}

// NewLang creates an initialized Lang node
func NewLang() *Lang {
	n := &Lang{}
	n.ID = <-ids
	return n
}

// NewLangWithContents creates an initialized Lang node with the given contents
func NewLangWithContents(n *Lang) *Lang {
	n.ID = <-ids
	return n
}

// IsLang returns the value of the element and true, if the element is a Lang
// node.
func IsLang(elt Node) (*Lang, bool) {
	lang, ok := elt.(*Lang)
	return lang, ok
}

// Type returns the type of the node
func (l *Lang) Type() Type {
	return TypeLang
}

// A Penalty is a valid horizontal or vertical break point. The higher the
// penalty the less likely the break occurs at this penalty. Anything below
// or equal -10000 is considered a forced break, anything higher than or
// equal to 10000 is considered a disallowed break.
type Penalty struct {
	basenode
	Penalty int             // Value
	Width   bag.ScaledPoint // Width of the penalty
}

func (p *Penalty) String() string {
	return String(p)
}

// Next returns the following node or nil if no such node exists.
func (p *Penalty) Next() Node {
	return p.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (p *Penalty) Prev() Node {
	return p.prev
}

// SetNext sets the following node.
func (p *Penalty) SetNext(n Node) {
	p.next = n
}

// SetPrev sets the preceding node.
func (p *Penalty) SetPrev(n Node) {
	p.prev = n
}

// GetID returns the node id
func (p *Penalty) GetID() int {
	return p.ID
}

// Name returns the name of the node
func (p *Penalty) Name() string {
	return "penalty"
}

// Type returns the type of the node
func (p *Penalty) Type() Type {
	return TypePenalty
}

// Copy creates a deep copy of the node.
func (p *Penalty) Copy() Node {
	n := NewPenalty()
	n.Penalty = p.Penalty
	n.Width = p.Width
	return n
}

// NewPenalty creates an initialized Penalty node
func NewPenalty() *Penalty {
	n := &Penalty{}
	n.ID = <-ids
	return n
}

// IsPenalty returns the value of the element and true, if the element is a Penalty
// node.
func IsPenalty(elt Node) (*Penalty, bool) {
	Penalty, ok := elt.(*Penalty)
	return Penalty, ok
}

// A Rule is a node represents a colored rectangular area.
type Rule struct {
	basenode
	// PDF code that gets output before the rule.
	Pre string
	// PDF Code after drawing the rule.
	Post string
	// Hide makes the rule invisible, no colored area is drawn. Used to make Pre
	// and Post appear in the output with the given dimensions.
	Hide   bool
	Width  bag.ScaledPoint
	Height bag.ScaledPoint
	Depth  bag.ScaledPoint
}

func (r *Rule) String() string {
	return String(r)
}

// Next returns the following node or nil if no such node exists.
func (r *Rule) Next() Node {
	return r.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (r *Rule) Prev() Node {
	return r.prev
}

// SetNext sets the following node.
func (r *Rule) SetNext(n Node) {
	r.next = n
}

// SetPrev sets the preceding node.
func (r *Rule) SetPrev(n Node) {
	r.prev = n
}

// GetID returns the node id
func (r *Rule) GetID() int {
	return r.ID
}

// Name returns the name of the node
func (r *Rule) Name() string {
	return "rule"
}

// Type returns the type of the node
func (r *Rule) Type() Type {
	return TypeRule
}

// Copy creates a deep copy of the node.
func (r *Rule) Copy() Node {
	n := NewRule()
	n.Width = r.Width
	n.Height = r.Height
	n.Depth = r.Depth
	return n
}

// NewRule creates an initialized Rule node
func NewRule() *Rule {
	n := &Rule{}
	n.ID = <-ids
	return n
}

// IsRule returns the value of the element and true, if the element is a Rule
// node.
func IsRule(elt Node) (*Rule, bool) {
	rule, ok := elt.(*Rule)
	return rule, ok
}

// PDFDataOutput defines the location of inserted PDF data.
type PDFDataOutput int

// ActionType represents a start/stop action such as a PDF link.
type ActionType int

const (
	// PDFOutputNone ignores any movement commands.
	PDFOutputNone PDFDataOutput = iota
	// PDFOutputHere inserts ET and moves to current position before inserting
	// the PDF data.
	PDFOutputHere
	// PDFOutputDirect inserts the PDF data without leaving the text mode with ET.
	PDFOutputDirect
	// PDFOutputPage inserts ET before writing the PDF data.
	PDFOutputPage
	// PDFOutputLowerLeft moves to the lower left corner before inserting the
	// PDF data.
	PDFOutputLowerLeft
)

const (
	// ActionNone represents no special action
	ActionNone ActionType = iota
	// ActionHyperlink represents a hyperlink.
	ActionHyperlink
	// ActionDest insets a PDF destination.
	ActionDest
	// ActionUserSetting allows user defined settings.
	ActionUserSetting
)

func (at ActionType) String() string {
	switch at {
	case ActionNone:
		return "ActionNone"
	case ActionHyperlink:
		return "ActionHyperlink"
	case ActionDest:
		return "ActionDest"
	case ActionUserSetting:
		return "ActionUserSetting"
	default:
		return "other action"
	}
}

// StartStopFunc is the type of the callback when this node is encountered in the
// node list. The returned string (if not empty) gets written to the PDF.
type StartStopFunc func(thisnode Node) string

// A StartStop is a paired node type used for color switches, hyperlinks and
// such.
type StartStop struct {
	basenode
	Action          ActionType
	StartNode       *StartStop
	Position        PDFDataOutput
	ShipoutCallback StartStopFunc
	// Value contains action specific contents
	Value any
}

func (d *StartStop) String() string {
	return String(d)
}

// NewStartStop creates an initialized Start node
func NewStartStop() *StartStop {
	n := &StartStop{}
	n.ID = <-ids
	return n
}

// Next returns the following node or nil if no such node exists.
func (d *StartStop) Next() Node {
	return d.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (d *StartStop) Prev() Node {
	return d.prev
}

// SetNext sets the following node.
func (d *StartStop) SetNext(n Node) {
	d.next = n
}

// SetPrev sets the preceding node.
func (d *StartStop) SetPrev(n Node) {
	d.prev = n
}

// GetID returns the node id
func (d *StartStop) GetID() int {
	return d.ID
}

// Name returns the name of the node
func (d *StartStop) Name() string {
	return "startstop"
}

// Type returns the type of the node
func (d *StartStop) Type() Type {
	return TypeStartStop
}

// Copy creates a deep copy of the node.
func (d *StartStop) Copy() Node {
	n := NewStartStop()
	return n
}

// A VList is a vertical list.
type VList struct {
	basenode
	Width    bag.ScaledPoint
	Height   bag.ScaledPoint
	Depth    bag.ScaledPoint
	GlueSet  float64
	GlueSign uint8
	ShiftX   bag.ScaledPoint
	List     Node
}

func (v *VList) String() string {
	return "vlist"
}

// Next returns the following node or nil if no such node exists.
func (v *VList) Next() Node {
	return v.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (v *VList) Prev() Node {
	return v.prev
}

// SetNext sets the following node.
func (v *VList) SetNext(n Node) {
	v.next = n
}

// SetPrev sets the preceding node.
func (v *VList) SetPrev(n Node) {
	v.prev = n
}

// GetID returns the node id
func (v *VList) GetID() int {
	return v.ID
}

// Name returns the name of the node
func (v *VList) Name() string {
	return "vlist"
}

// Type returns the type of the node
func (v *VList) Type() Type {
	return TypeVList
}

// Copy creates a deep copy of the node.
func (v *VList) Copy() Node {
	n := NewVList()
	n.Width = v.Width
	n.Height = v.Height
	n.Depth = v.Depth
	n.GlueSet = v.GlueSet
	n.GlueSign = v.GlueSign
	n.ShiftX = v.ShiftX
	n.List = CopyList(v.List)
	return n
}

// NewVList creates an initialized VList node
func NewVList() *VList {
	n := &VList{}
	n.ID = <-ids
	return n
}

// IsVList returns the value of the element and true, if the element is a VList node.
func IsVList(elt Node) (*VList, bool) {
	vlist, ok := elt.(*VList)
	return vlist, ok
}

// An Image contains a reference to the image object.
type Image struct {
	basenode
	Width  bag.ScaledPoint
	Height bag.ScaledPoint
	Img    *image.Image
}

func (img *Image) String() string {
	return "image"
}

// Next returns the following node or nil if no such node exists.
func (img *Image) Next() Node {
	return img.next
}

// Prev returns the node preceding this node or nil if no such node exists.
func (img *Image) Prev() Node {
	return img.prev
}

// SetNext sets the following node.
func (img *Image) SetNext(n Node) {
	img.next = n
}

// SetPrev sets the preceding node.
func (img *Image) SetPrev(n Node) {
	img.prev = n
}

// GetID returns the node id
func (img *Image) GetID() int {
	return img.ID
}

// Name returns the name of the node
func (img *Image) Name() string {
	return "image"
}

// Type returns the type of the node
func (img *Image) Type() Type {
	return TypeImage
}

// Copy creates a deep copy of the node.
func (img *Image) Copy() Node {
	n := NewImage()
	n.Width = img.Width
	n.Height = img.Height
	n.Img = img.Img
	return n
}

// NewImage creates an initialized Image node
func NewImage() *Image {
	n := &Image{}
	n.ID = <-ids
	return n
}

// IsImage returns the value of the element and true, if the element is a Image node.
func IsImage(elt Node) (*Image, bool) {
	img, ok := elt.(*Image)
	return img, ok
}
