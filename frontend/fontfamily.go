package frontend

import (
	"fmt"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
)

var (
	// ErrEmptyFF is returned when requesting a font from an empty font family.
	ErrEmptyFF = fmt.Errorf("no face defined in the font family yet")
	// ErrUnfulfilledFamilyRequest is returned when the GetFace method does
	// cannot find the exact family member but has to chose another member.
	ErrUnfulfilledFamilyRequest = fmt.Errorf("the font family does not have the exact requested member")
)

// NewFontFamily creates a new font family for bundling fonts.
func (fe *Document) NewFontFamily(name string) *FontFamily {
	ff := &FontFamily{
		ID:   len(fe.FontFamilies),
		Name: name,
	}
	fe.FontFamilies = append(fe.FontFamilies, ff)
	return ff
}

// GetFontFamily returns the font family with the given id.
func (fe *Document) GetFontFamily(id int) *FontFamily {
	if id >= len(fe.FontFamilies) {
		return nil
	}
	return fe.FontFamilies[id]
}

// LoadFace loads a font from a TrueType or OpenType collection.
func (fe *Document) LoadFace(fs *FontSource) (*pdf.Face, error) {
	bag.Logger.Debugf("LoadFace %s", fs)
	if fs.face != nil {
		return fs.face, nil
	}

	f, err := fe.Doc.LoadFace(fs.Source, fs.Index)
	if err != nil {
		return nil, err
	}
	fs.face = f
	return f, nil
}

// FontSource defines a mapping of name to a font source including the font features.
type FontSource struct {
	Name         string
	FontFeatures []string
	Source       string
	// The sub font index within the font file.
	Index int
	// Used to save a face once it is loaded.
	face *pdf.Face
}

func (fs *FontSource) String() string {
	return fmt.Sprintf("%s->%s:%d", fs.Name, fs.Source, fs.Index)
}

// FontFamily is a struct that keeps font with different weights and styles together.
type FontFamily struct {
	ID           int
	Name         string
	familyMember map[int]map[FontStyle]*FontSource
}

// AddMember adds a member to the font family.
func (ff *FontFamily) AddMember(fontsource *FontSource, weight int, style FontStyle) {
	if ff.familyMember == nil {
		ff.familyMember = make(map[int]map[FontStyle]*FontSource)
	}
	if ff.familyMember[weight] == nil {
		ff.familyMember[weight] = make(map[FontStyle]*FontSource)
	}
	ff.familyMember[weight][style] = fontsource
}

// GetFontSource tries to get the face closest to the requested face.
func (ff *FontFamily) GetFontSource(weight int, style FontStyle) (*FontSource, error) {
	if ff.familyMember == nil {
		return nil, ErrEmptyFF
	}
	if ff.familyMember[weight] == nil {
		// todo: implement algorithm as described in CSS/font-weight
		return nil, ErrUnfulfilledFamilyRequest
	}
	if ff.familyMember[weight][style] == nil {
		// todo: implement algorithm to get different style?
		return nil, ErrUnfulfilledFamilyRequest
	}
	return ff.familyMember[weight][style], nil

}
