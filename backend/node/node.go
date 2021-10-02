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

type basenode struct {
	ID int
}

// NewElement creates a list element from the node type. You must ensure that
// the val is a valid node type.
func NewElement(val interface{}) *Node {
	return &Node{Value: val}
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

// NewDiscWithContents creates an initialized Disc node with the given contents
func NewDiscWithContents(n *Disc) *Disc {
	n.ID = <-ids
	return n
}

// IsDisc retuns the value of the element and true, if the element is a Disc
// node.
func IsDisc(elt *Node) (*Disc, bool) {
	Disc, ok := elt.Value.(*Disc)
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
	return fmt.Sprintf("glyph: %s (font: %s)", g.Components, g.Font.Face.InternalName())
}

// NewGlyph returns an initialized Glyph
func NewGlyph() *Glyph {
	n := &Glyph{}
	n.ID = <-ids
	return n
}

// IsGlyph returns the value of the element and true, if the element is a Glyph
// node.
func IsGlyph(elt *Node) (*Glyph, bool) {
	n, ok := elt.Value.(*Glyph)
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
func IsGlue(elt *Node) (*Glue, bool) {
	n, ok := elt.Value.(*Glue)
	return n, ok
}

// A HList is a horizontal list.
type HList struct {
	basenode
	Width  bag.ScaledPoint
	Height bag.ScaledPoint
	List   *Nodelist
}

func (h *HList) String() string {
	return "hlist"
}

// Head returns the head of the list
func (h *HList) Head() *Node {
	return h.List.Front()
}

// NewHList creates an initialized HList node
func NewHList() *HList {
	n := &HList{}
	n.ID = <-ids
	return n
}

// IsHList retuns the value of the element and true, if the element is a HList
// node.
func IsHList(elt *Node) (*HList, bool) {
	hlist, ok := elt.Value.(*HList)
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
func IsLang(elt *Node) (*Lang, bool) {
	lang, ok := elt.Value.(*Lang)
	return lang, ok
}

// A VList is a horizontal list.
type VList struct {
	basenode
	Width     bag.ScaledPoint
	Height    bag.ScaledPoint
	List      *Nodelist
	FirstFont *font.Font
}

func (v *VList) String() string {
	return "vlist"
}

// Head returns the head of the list
func (v *VList) Head() *Node {
	return v.List.Front()
}

// NewVList creates an initialized VList node
func NewVList() *VList {
	n := &VList{}
	n.List = NewNodelist()
	n.ID = <-ids
	return n
}

// IsVList retuns the value of the element and true, if the element is a VList node.
func IsVList(elt *Node) (*VList, bool) {
	vlist, ok := elt.Value.(*VList)
	return vlist, ok
}

// An Image is a horizontal list.
type Image struct {
	basenode
	Width  bag.ScaledPoint
	Height bag.ScaledPoint
	Img    *image.Image
}

func (n *Image) String() string {
	return "image"
}

// NewImage creates an initialized Image node
func NewImage() *Image {
	n := &Image{}
	n.ID = <-ids
	return n
}

// IsImage retuns the value of the element and true, if the element is a Image node.
func IsImage(elt *Node) (*Image, bool) {
	img, ok := elt.Value.(*Image)
	return img, ok
}
