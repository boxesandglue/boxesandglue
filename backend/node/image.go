package node

import (
	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// An Image contains a reference to the image object.
type Image struct {
	basenode
	ImageFile  *pdf.Imagefile
	Width      bag.ScaledPoint
	Height     bag.ScaledPoint
	PageNumber int // Requested page number
	Used       bool
}

func (img *Image) String() string {
	return "image"
}

// Sizes returns the image's bounding-box dimensions; depth is always zero.
func (img *Image) Sizes(Direction) (w, h, d bag.ScaledPoint) {
	return img.Width, img.Height, 0
}

// DebugAttributes returns the image's filename and geometry.
func (img *Image) DebugAttributes() ([]kv, H) {
	filename := "(image object not set)"
	if img.ImageFile != nil {
		filename = img.ImageFile.Filename
	}
	return []kv{
		{key: "id", value: img.ID},
		{key: "filename", value: filename},
		{key: "wd", value: img.Width},
		{key: "ht", value: img.Height},
	}, img.Attributes
}

// Copy creates a deep copy of the node.
func (img *Image) Copy() Node {
	n := NewImage()
	n.Width = img.Width
	n.Height = img.Height
	n.ImageFile = img.ImageFile
	n.PageNumber = img.PageNumber
	n.Used = img.Used
	return n
}

// NewImage creates an initialized Image node
func NewImage() *Image {
	n := imageSlab.alloc()
	n.ID = newID()
	n.typ = TypeImage
	return n
}
