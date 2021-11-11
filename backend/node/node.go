package node

import (
	"fmt"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/image"
	"github.com/speedata/boxesandglue/backend/lang"
)

var (
	ids chan int
)

// Node represents any kind of node
type Node interface {
	Next() Node
	Prev() Node
	SetNext(Node)
	SetPrev(Node)
	GetID() int
	Name() string
}

// String returns a string representation of the node n and the previous and next node.
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
		extrainfo = fmt.Sprintf(": %spt plus %d", t.Width, t.Stretch)
	case *Glyph:
		extrainfo = fmt.Sprintf(": %s (font: %s)", t.Components, t.Font.Face.InternalName())

	}
	return fmt.Sprintf(" %s <- %s %d -> %s%s", pr, n.Name(), n.GetID(), nx, extrainfo)

}

type basenode struct {
	next Node
	prev Node
	ID   int
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
func IsNode(arg interface{}) bool {
	switch arg.(type) {
	case *Disc, *Glyph, *Glue, *Image, *HList, *Lang, *VList:
		return true
	}
	return false
}

// A Disc is a hyphenation point.
type Disc struct {
	basenode
}

func (d *Disc) String() string {
	return "disc"
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

// Prev returns the node preceeding this node or nil if no such node exists.
func (d *Disc) Prev() Node {
	return d.prev
}

// SetNext sets the following node.
func (d *Disc) SetNext(n Node) {
	d.next = n
}

// SetPrev sets the preceeding node.
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

// NewDiscWithContents creates an initialized Disc node with the given contents
func NewDiscWithContents(n *Disc) *Disc {
	n.ID = <-ids
	return n
}

// IsDisc retuns the value of the element and true, if the element is a Disc
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
	Hyphenate  bool
}

func (g *Glyph) String() string {
	return String(g)
}

// Next returns the following node or nil if no such node exists.
func (g *Glyph) Next() Node {
	return g.next
}

// Prev returns the node preceeding this node or nil if no such node exists.
func (g *Glyph) Prev() Node {
	return g.prev
}

// SetNext sets the following node.
func (g *Glyph) SetNext(n Node) {
	g.next = n
}

// SetPrev sets the preceeding node.
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

// A Glue node has the value of a shrinking and stretching space
type Glue struct {
	basenode
	Width        bag.ScaledPoint
	Stretch      int
	Shrink       int
	StretchOrder int
	ShrinkOrder  int
}

func (g *Glue) String() string {
	return String(g)
}

// Next returns the following node or nil if no such node exists.
func (g *Glue) Next() Node {
	return g.next
}

// Prev returns the node preceeding this node or nil if no such node exists.
func (g *Glue) Prev() Node {
	return g.prev
}

// SetNext sets the following node.
func (g *Glue) SetNext(n Node) {
	g.next = n
}

// SetPrev sets the preceeding node.
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

// NewGlue creates an initialized Glue node
func NewGlue() *Glue {
	n := &Glue{}
	n.ID = <-ids
	return n
}

// IsGlue retuns the value of the element and true, if the element is a Glue
// node.
func IsGlue(elt Node) (*Glue, bool) {
	n, ok := elt.(*Glue)
	return n, ok
}

// A HList is a horizontal list.
type HList struct {
	basenode
	Width  bag.ScaledPoint
	Height bag.ScaledPoint
	List   Node
}

func (h *HList) String() string {
	return "hlist"
}

// Head returns the head of the list
func (h *HList) Head() Node {
	return h.List
}

// Next returns the following node or nil if no such node exists.
func (h *HList) Next() Node {
	return h.next
}

// Prev returns the node preceeding this node or nil if no such node exists.
func (h *HList) Prev() Node {
	return h.prev
}

// SetNext sets the following node.
func (h *HList) SetNext(n Node) {
	h.next = n
}

// SetPrev sets the preceeding node.
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

// NewHList creates an initialized HList node
func NewHList() *HList {
	n := &HList{}
	n.ID = <-ids
	return n
}

// IsHList retuns the value of the element and true, if the element is a HList
// node.
func IsHList(elt Node) (*HList, bool) {
	hlist, ok := elt.(*HList)
	return hlist, ok
}

// A Lang is a node that sets the current language.
type Lang struct {
	basenode
	Lang *lang.Lang
}

func (l *Lang) String() string {
	return "lang: " + l.Lang.Name
}

// Next returns the following node or nil if no such node exists.
func (l *Lang) Next() Node {
	return l.next
}

// Prev returns the node preceeding this node or nil if no such node exists.
func (l *Lang) Prev() Node {
	return l.prev
}

// SetNext sets the following node.
func (l *Lang) SetNext(n Node) {
	l.next = n
}

// SetPrev sets the preceeding node.
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

// IsLang retuns the value of the element and true, if the element is a Lang
// node.
func IsLang(elt Node) (*Lang, bool) {
	lang, ok := elt.(*Lang)
	return lang, ok
}

// A Penalty is a node that sets information about a place to break a list.
type Penalty struct {
	basenode
	Penalty int
	Flagged bool
	Width   bag.ScaledPoint
}

func (p *Penalty) String() string {
	return fmt.Sprintf("Penalty: %d", p.Penalty)
}

// Next returns the following node or nil if no such node exists.
func (p *Penalty) Next() Node {
	return p.next
}

// Prev returns the node preceeding this node or nil if no such node exists.
func (p *Penalty) Prev() Node {
	return p.prev
}

// SetNext sets the following node.
func (p *Penalty) SetNext(n Node) {
	p.next = n
}

// SetPrev sets the preceeding node.
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

// NewPenalty creates an initialized Penalty node
func NewPenalty() *Penalty {
	n := &Penalty{}
	n.ID = <-ids
	return n
}

// IsPenalty retuns the value of the element and true, if the element is a Penalty
// node.
func IsPenalty(elt Node) (*Penalty, bool) {
	Penalty, ok := elt.(*Penalty)
	return Penalty, ok
}

// A VList is a horizontal list.
type VList struct {
	basenode
	Width     bag.ScaledPoint
	Height    bag.ScaledPoint
	FirstFont *font.Font
	List      Node
}

func (v *VList) String() string {
	return "vlist"
}

// Head returns the head of the list
func (v *VList) Head() Node {
	return v.List
}

// Next returns the following node or nil if no such node exists.
func (v *VList) Next() Node {
	return v.next
}

// Prev returns the node preceeding this node or nil if no such node exists.
func (v *VList) Prev() Node {
	return v.prev
}

// SetNext sets the following node.
func (v *VList) SetNext(n Node) {
	v.next = n
}

// SetPrev sets the preceeding node.
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

// NewVList creates an initialized VList node
func NewVList() *VList {
	n := &VList{}
	n.ID = <-ids
	return n
}

// IsVList retuns the value of the element and true, if the element is a VList node.
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

// Prev returns the node preceeding this node or nil if no such node exists.
func (img *Image) Prev() Node {
	return img.prev
}

// SetNext sets the following node.
func (img *Image) SetNext(n Node) {
	img.next = n
}

// SetPrev sets the preceeding node.
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

// NewImage creates an initialized Image node
func NewImage() *Image {
	n := &Image{}
	n.ID = <-ids
	return n
}

// IsImage retuns the value of the element and true, if the element is a Image node.
func IsImage(elt Node) (*Image, bool) {
	img, ok := elt.(*Image)
	return img, ok
}
