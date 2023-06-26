package frontend

import (
	"fmt"
	"strconv"
	"strings"

	pdf "github.com/speedata/baseline-pdf"
	"github.com/speedata/boxesandglue/backend/bag"
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
	bag.Logger.Infof("Define font family %q (id %d)", name, len(fe.FontFamilies))
	ff := &FontFamily{
		ID:   len(fe.FontFamilies),
		Name: name,
	}
	fe.FontFamilies[name] = ff
	return ff
}

// FindFontFamily returns the font family with the given name or nil if there is
// no font family with this name.
func (fe *Document) FindFontFamily(name string) *FontFamily {
	return fe.FontFamilies[name]
}

// DefineFontFamilyAlias defines the font family with the new name.
func (fe *Document) DefineFontFamilyAlias(ff *FontFamily, alias string) {
	bag.Logger.Infof("Define font family alias %q", alias)
	fe.FontFamilies[alias] = ff
}

// LoadFace loads a font from a TrueType or OpenType collection. It takes the
// face from the cache if the face has been loaded.
func (fe *Document) LoadFace(fs *FontSource) (*pdf.Face, error) {
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
	SizeAdjust   float64 // 1 - SizeAdjust is the relative adjustment.
	// The sub font index within the font file.
	Index int
	// Used to save a face once it is loaded.
	face *pdf.Face
}

func (fs *FontSource) String() string {
	name := fs.Name
	if name == "" {
		name = "-"
	}
	return fmt.Sprintf("%s->%s:%d (feat: %s)", name, fs.Source, fs.Index, fs.FontFeatures)
}

// FontFamily is a struct that keeps font with different weights and styles together.
type FontFamily struct {
	ID           int
	Name         string
	familyMember map[FontWeight]map[FontStyle]*FontSource
}

// AddMember adds a member to the font family.
func (ff *FontFamily) AddMember(fontsource *FontSource, weight FontWeight, style FontStyle) error {
	bag.Logger.Debugf("add member to ff (id %d) weight %s, style %s", ff.ID, weight, style)
	if fontsource == nil {
		return fmt.Errorf("Font source is nil")
	}
	if ff.familyMember == nil {
		ff.familyMember = make(map[FontWeight]map[FontStyle]*FontSource)
	}
	if ff.familyMember[weight] == nil {
		ff.familyMember[weight] = make(map[FontStyle]*FontSource)
	}
	ff.familyMember[weight][style] = fontsource
	return nil
}

// GetFontSource tries to get the face closest to the requested face.
func (ff *FontFamily) GetFontSource(weight FontWeight, style FontStyle) (*FontSource, error) {
	if ff == nil {
		return nil, fmt.Errorf("no font family specified")
	}

	if ff.familyMember == nil {
		return nil, ErrEmptyFF
	}
	if ff.familyMember[weight] == nil {
		if weight >= 400 && weight <= 500 {
			for i := weight; i <= 500; i++ {
				if ff.familyMember[i] != nil {
					weight = i
					goto found
				}
			}
			for i := weight; i > 0; i-- {
				if ff.familyMember[i] != nil {
					weight = i
					goto found
				}
			}
			for i := weight; i < 1000; i++ {
				if ff.familyMember[i] != nil {
					weight = i
					goto found
				}
			}
		} else if weight < 400 {
			for i := weight; i > 0; i-- {
				if ff.familyMember[i] != nil {
					weight = i
					goto found
				}
			}
			for i := weight; i < 1000; i++ {
				if ff.familyMember[i] != nil {
					weight = i
					goto found
				}
			}
		} else {
			for i := weight; i < 1000; i++ {
				if ff.familyMember[i] != nil {
					weight = i
					goto found
				}
			}
			for i := weight; i > 0; i-- {
				if ff.familyMember[i] != nil {
					weight = i
					goto found
				}
			}
		}
		return nil, ErrUnfulfilledFamilyRequest
	}
found:
	ffMemberWeight := ff.familyMember[weight]
	if ff := ffMemberWeight[style]; ff != nil {
		return ff, nil
	}
	keys := []string{}
	for k := range ffMemberWeight {
		keys = append(keys, k.String())
	}
	bag.Logger.Warnf("Style %s not found in font family %s. Known styles for weight %s are %s", style, ff.Name, weight, strings.Join(keys, ", "))
	// fallback to normal
	if ff := ffMemberWeight[FontStyleNormal]; ff != nil {
		return ff, nil
	}
	return nil, ErrUnfulfilledFamilyRequest
}

// ResolveFontWeight returns a FontWeight based on the string fw. For example
// bold is converted to font weight 700.
func ResolveFontWeight(fw string, inheritedValue FontWeight) FontWeight {
	switch strings.ToLower(fw) {
	case "thin", "hairline":
		return FontWeight100
	case "extra light", "ultra light":
		return FontWeight200
	case "light":
		return FontWeight300
	case "normal":
		return FontWeight400
	case "medium":
		return FontWeight500
	case "semi bold", "demi bold":
		return FontWeight600
	case "bold":
		return FontWeight700
	case "extra bold", "ultra bold":
		return FontWeight800
	case "black", "heavy":
		return FontWeight900
	case "bolder":
		if inheritedValue < 400 {
			return FontWeight400
		} else if inheritedValue < 600 {
			return FontWeight700
		} else {
			return FontWeight900
		}
	}

	i, err := strconv.Atoi(fw)
	if err != nil {
		bag.Logger.Errorf("resolve font size: cannot convert %s to int", fw)
		return FontWeight400
	}

	return FontWeight(i)
}

// ResolveFontStyle parses the string fs and returns a font style.
func ResolveFontStyle(fs string) FontStyle {
	switch strings.ToLower(fs) {
	case "italic":
		return FontStyleItalic
	case "normal":
		return FontStyleNormal
	case "oblique":
		return FontStyleOblique
	}
	return FontStyleNormal
}

func (ff FontFamily) String() string {
	ret := []string{}
	ret = append(ret, fmt.Sprintf("id: %d, name: %s", ff.ID, ff.Name))
	return strings.Join(ret, "")
}
