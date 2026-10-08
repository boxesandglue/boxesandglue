package node

import (
	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// An Image contains a reference to the image object.
//
// Height is the extent above the baseline; Depth is the extent below.
// The visible rendered height equals Height + Depth. Splitting an image
// into Height/Depth supports CSS-style vertical-align: text-top, where
// the image's top sits at the parent's font ascent and its lower portion
// extends below the baseline.
type Image struct {
	basenode
	ImageFile  *pdf.Imagefile
	Width      bag.ScaledPoint
	Height     bag.ScaledPoint
	Depth      bag.ScaledPoint
	PageNumber int // Requested page number
	Used       bool
	// Crop, if set, is the region of the image to show. It fills the
	// image's box, and the image is clipped to the box.
	Crop *ImageCrop
}

// ImageCrop is a region of an image in the units of its natural size: pixels
// for a bitmap, points of the page box for a PDF page, and points of the width
// and height for an SVG document. X and Y are measured from the image's top
// left corner.
type ImageCrop struct {
	X, Y, Width, Height float64
}

func (img *Image) String() string {
	return "image"
}

// Sizes returns the image's bounding-box dimensions.
func (img *Image) Sizes(Direction) (w, h, d bag.ScaledPoint) {
	return img.Width, img.Height, img.Depth
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
		{key: "dp", value: img.Depth},
	}, img.Attributes
}

// Copy creates a deep copy of the node.
func (img *Image) Copy() Node {
	n := NewImage()
	n.Width = img.Width
	n.Height = img.Height
	n.Depth = img.Depth
	n.ImageFile = img.ImageFile
	n.PageNumber = img.PageNumber
	n.Used = img.Used
	if img.Crop != nil {
		c := *img.Crop
		n.Crop = &c
	}
	n.Attributes = cloneAttributes(img.Attributes)
	return n
}

// NewImage creates an initialized Image node
func NewImage() *Image {
	n := imageSlab.alloc()
	n.ID = newID()
	n.typ = TypeImage
	return n
}
