package frontend

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
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
	bag.Logger.Info("Define font family", "name", name, "id", len(fe.FontFamilies))
	ff := &FontFamily{
		ID:   len(fe.FontFamilies),
		Name: name,
		doc:  fe,
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
	bag.Logger.Info("Define font family alias", "alias", alias)
	fe.FontFamilies[alias] = ff
}

// LoadFace loads a font from a TrueType or OpenType collection. It takes the
// face from the cache if the face has been loaded.
func (fe *Document) LoadFace(fs *FontSource) (*pdf.Face, error) {
	if fs.face != nil {
		return fs.face, nil
	}
	var err error
	var f *pdf.Face
	if fs.Location == "" {
		f, err = fe.Doc.LoadFaceFromData(fs.Data, fs.Index)
		if err != nil {
			return nil, err
		}
	} else {
		f, err = fe.Doc.LoadFace(fs.Location, fs.Index)
		if err != nil {
			return nil, err
		}
	}

	// Pass variation settings to the Face for PDF instancing
	if fs.VariationSettings != nil {
		f.VariationSettings = fs.VariationSettings
	}

	fs.face = f
	return f, nil
}

// LoadFaceWithVariations loads a font face with specific variation settings.
// If the variations differ from the FontSource defaults, a new face is created
// and cached separately. This allows different text runs to use the same font
// with different variation settings.
func (fe *Document) LoadFaceWithVariations(fs *FontSource, variations map[string]float64) (*pdf.Face, error) {
	// If no inline variations, use the standard LoadFace
	if len(variations) == 0 {
		return fe.LoadFace(fs)
	}

	// Check if variations match FontSource defaults - if so, use standard LoadFace
	if len(variations) == len(fs.VariationSettings) {
		allMatch := true
		for k, v := range variations {
			if fsv, ok := fs.VariationSettings[k]; !ok || fsv != v {
				allMatch = false
				break
			}
		}
		if allMatch {
			return fe.LoadFace(fs)
		}
	}

	// Create a cache key from FontSource location/name and variations
	cacheKey := fs.Location
	if cacheKey == "" {
		cacheKey = fs.Name
	}
	cacheKey = fmt.Sprintf("%s:%d", cacheKey, fs.Index)
	// Add sorted variation settings to key for consistency
	keys := make([]string, 0, len(variations))
	for k := range variations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cacheKey += fmt.Sprintf(":%s=%.2f", k, variations[k])
	}

	// Check cache
	if face, ok := fe.variationFaces[cacheKey]; ok {
		return face, nil
	}

	// Load a new face for this variation combination
	// We bypass the document's LoadFace cache to create a separate face for each variation
	var f *pdf.Face
	var err error
	if fs.Location == "" {
		f, err = fe.Doc.PDFWriter.NewFaceFromData(fs.Data, fs.Index)
	} else {
		f, err = fe.Doc.PDFWriter.LoadFace(fs.Location, fs.Index)
	}
	if err != nil {
		return nil, err
	}

	// Add to document's face list for PDF finalization
	fe.Doc.Faces = append(fe.Doc.Faces, f)

	// Set the specific variations
	f.VariationSettings = variations

	// Cache for future use
	fe.variationFaces[cacheKey] = f

	return f, nil
}

// AddDataToFontsource adds the font data to the font source.
func (fe *Document) AddDataToFontsource(fs *FontSource, fontname string) error {
	savedFS, ok := fe.fontlocal[fontname]
	if !ok {
		return fmt.Errorf("local font %q not found", fontname)
	}
	fs.Data = savedFS.Data
	return nil
}

// FontSource defines a mapping of name to a font source including the font features.
type FontSource struct {
	VariationSettings map[string]float64 // axis tag -> value (e.g., "wght" -> 700)
	// Used to save a face once it is loaded.
	face         *pdf.Face
	Name         string
	Location     string
	FontFeatures []string
	Data         []byte
	SizeAdjust   float64 // 1 - SizeAdjust is the relative adjustment.
	// The sub font index within the font file.
	Index int
}

func (fs *FontSource) String() string {
	name := fs.Name
	if name == "" {
		name = "-"
	}
	return fmt.Sprintf("%s->%s:%d (feat: %s, var: %v)", name, fs.Location, fs.Index, fs.FontFeatures, fs.VariationSettings)
}

// FontFamily is a struct that keeps font with different weights and styles together.
type FontFamily struct {
	doc          *Document
	familyMember map[FontWeight]map[FontStyle]*FontSource
	// rangeMembers are @font-face entries that cover a weight range (CSS
	// Fonts 4 `font-weight: 200 900`), typically variable fonts with a
	// wght axis. They are matched by containment and instantiated per
	// used weight.
	rangeMembers []rangeMember
	// rangeInstances caches the per-weight FontSource derived from a
	// range member, so repeated lookups return pointer-identical sources:
	// the face cache and the coverage cache key by identity.
	rangeInstances map[rangeInstanceKey]*FontSource
	Name           string
	ID             int
}

// rangeMember is a family member covering [min, max] instead of a single
// weight.
type rangeMember struct {
	fs       *FontSource
	min, max FontWeight
	style    FontStyle
}

type rangeInstanceKey struct {
	base *FontSource
	w    FontWeight
}

// AddMember adds a member to the font family.
func (ff *FontFamily) AddMember(fontsource *FontSource, weight FontWeight, style FontStyle) error {
	ff.doc.fontlocal[fontsource.Name] = fontsource
	bag.Logger.Debug("Add member to font family", "id", ff.ID, "weight", weight, "style", style, "source", fontsource)
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

// AddMemberRange adds a member that covers the weight range [weightMin,
// weightMax] (CSS Fonts 4 range syntax in @font-face, e.g. `font-weight:
// 200 900`), typically a variable font with a wght axis. A request for a
// weight inside the range resolves to a per-weight instance of the font
// source whose "wght" variation axis is pinned to the requested weight.
// Point members (AddMember) take precedence at their exact weight.
func (ff *FontFamily) AddMemberRange(fontsource *FontSource, weightMin, weightMax FontWeight, style FontStyle) error {
	if fontsource == nil {
		return fmt.Errorf("Font source is nil")
	}
	if weightMin > weightMax {
		weightMin, weightMax = weightMax, weightMin
	}
	if weightMin == weightMax {
		return ff.AddMember(fontsource, weightMin, style)
	}
	ff.doc.fontlocal[fontsource.Name] = fontsource
	bag.Logger.Debug("Add range member to font family", "id", ff.ID, "min", weightMin, "max", weightMax, "style", style, "source", fontsource)
	ff.rangeMembers = append(ff.rangeMembers, rangeMember{fs: fontsource, min: weightMin, max: weightMax, style: style})
	return nil
}

// hasWeight reports whether a point member exists at w or a range member
// covers w.
func (ff *FontFamily) hasWeight(w FontWeight) bool {
	if ff.familyMember[w] != nil {
		return true
	}
	for _, r := range ff.rangeMembers {
		if w >= r.min && w <= r.max {
			return true
		}
	}
	return false
}

// stylesAt returns the style map in effect at weight w. Point members
// registered exactly at w take precedence; range members covering w fill
// in styles the point map does not provide, as per-weight instances.
func (ff *FontFamily) stylesAt(w FontWeight) map[FontStyle]*FontSource {
	m := ff.familyMember[w]
	var merged map[FontStyle]*FontSource
	for _, r := range ff.rangeMembers {
		if w < r.min || w > r.max {
			continue
		}
		if m[r.style] != nil || merged[r.style] != nil {
			continue
		}
		if merged == nil {
			merged = make(map[FontStyle]*FontSource, len(m)+1)
			maps.Copy(merged, m)
		}
		merged[r.style] = ff.instanceAt(r.fs, w)
	}
	if merged != nil {
		return merged
	}
	return m
}

// instanceAt returns the per-weight instance of a range member's font
// source: a copy with the "wght" variation axis pinned to w. Cached so
// repeated lookups return the same pointer.
func (ff *FontFamily) instanceAt(base *FontSource, w FontWeight) *FontSource {
	key := rangeInstanceKey{base: base, w: w}
	if inst, ok := ff.rangeInstances[key]; ok {
		return inst
	}
	vs := make(map[string]float64, len(base.VariationSettings)+1)
	maps.Copy(vs, base.VariationSettings)
	vs["wght"] = float64(w)
	inst := &FontSource{
		Name:              base.Name,
		Location:          base.Location,
		FontFeatures:      base.FontFeatures,
		Data:              base.Data,
		SizeAdjust:        base.SizeAdjust,
		Index:             base.Index,
		VariationSettings: vs,
	}
	if ff.rangeInstances == nil {
		ff.rangeInstances = make(map[rangeInstanceKey]*FontSource)
	}
	ff.rangeInstances[key] = inst
	return inst
}

// GetFontSource tries to get the face closest to the requested face.
func (ff *FontFamily) GetFontSource(weight FontWeight, style FontStyle) (*FontSource, error) {
	bag.Logger.Log(context.Background(), -8, "FontFamily#GetFontSource", "weight", weight, "style", style)
	if ff == nil {
		return nil, fmt.Errorf("no font family specified")
	}

	if ff.familyMember == nil && len(ff.rangeMembers) == 0 {
		return nil, ErrEmptyFF
	}
	if !ff.hasWeight(weight) {
		switch {
		case weight >= 400 && weight <= 500:
			for i := weight; i <= 500; i++ {
				if ff.hasWeight(i) {
					weight = i
					goto found
				}
			}
			for i := weight; i > 0; i-- {
				if ff.hasWeight(i) {
					weight = i
					goto found
				}
			}
			for i := weight; i < 1000; i++ {
				if ff.hasWeight(i) {
					weight = i
					goto found
				}
			}
		case weight < 400:
			for i := weight; i > 0; i-- {
				if ff.hasWeight(i) {
					weight = i
					goto found
				}
			}
			for i := weight; i < 1000; i++ {
				if ff.hasWeight(i) {
					weight = i
					goto found
				}
			}
		default:
			for i := weight; i < 1000; i++ {
				if ff.hasWeight(i) {
					weight = i
					goto found
				}
			}
			for i := weight; i > 0; i-- {
				if ff.hasWeight(i) {
					weight = i
					goto found
				}
			}
		}
		return nil, ErrUnfulfilledFamilyRequest
	}
found:
	ffMemberWeight := ff.stylesAt(weight)
	if ff := ffMemberWeight[style]; ff != nil {
		return ff, nil
	}
	keys := []string{}
	for k := range ffMemberWeight {
		keys = append(keys, k.String())
	}
	bag.Logger.Warn(fmt.Sprintf("Style %s not found in font family %s. Known styles for weight %s are %s", style, ff.Name, weight, strings.Join(keys, ", ")))
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
	case "normal", "regular":
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
		switch {
		case inheritedValue < 400:
			return FontWeight400
		case inheritedValue < 600:
			return FontWeight700
		default:
			return FontWeight900
		}
	}
	i, err := strconv.Atoi(fw)
	if err != nil {
		// Invalid values leave the inherited weight in place, like a
		// browser dropping an invalid declaration. The two-number form
		// is a common mix-up: it is only valid as an @font-face
		// descriptor, not as a property value, so give a targeted hint.
		if f := strings.Fields(fw); len(f) == 2 {
			_, err1 := strconv.Atoi(f[0])
			_, err2 := strconv.Atoi(f[1])
			if err1 == nil && err2 == nil {
				bag.Logger.Warn("font-weight range is only valid inside @font-face; use a single value on the element", "value", fw)
				return inheritedValue
			}
		}
		bag.Logger.Warn(fmt.Sprintf("resolve font weight: cannot convert %q to int", fw))
		return inheritedValue
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
