package node

import "testing"

// A copy has its own crop region.
func TestImageCopyCrop(t *testing.T) {
	img := NewImage()
	img.Crop = &ImageCrop{X: 1, Y: 2, Width: 3, Height: 4}
	cp := img.Copy().(*Image)
	if cp.Crop == nil || *cp.Crop != *img.Crop {
		t.Fatalf("copy has crop %v, want %v", cp.Crop, *img.Crop)
	}
	if cp.Crop == img.Crop {
		t.Error("copy shares the crop region")
	}
	if NewImage().Copy().(*Image).Crop != nil {
		t.Error("copy of an image without crop has one")
	}
}
