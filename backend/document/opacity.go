package document

import (
	"fmt"
	"io"
	"maps"
	"slices"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// Opacity is the attribute key of a VList that is painted as one object with
// a constant opacity, the value a float64 from 0 (invisible) to 1 (opaque).
// Unlike the alpha value of a color it applies to the composed result: text
// on a background of its own in the VList does not let the background shine
// through. The VList ends up in a Form XObject with a transparency group,
// together with the nodes marked PaintLast inside it, which are painted at
// its end. Formats without transparency (PDF/A-1, PDF/X-3) paint it opaque
// with a warning.
const Opacity = "opacity"

// groupForm is the content stream of a VList with an Opacity while it is
// painted. Marked content in it has MCIDs of its own, numbered from 0, which
// the ParentTree reaches through the /StructParents of the form.
type groupForm struct {
	content  *pdf.Object
	nextMCID int
	elements []*StructureElement // index is the MCID, as for a page
}

// groupState is what painting a group swaps out of the objectContext and puts
// back afterwards: the stream, the resources it collects and what the
// objectContext knows about the graphics state, which the Do of the form
// restores.
type groupState struct {
	s                 io.Writer
	form              *groupForm
	usedFaces         map[*pdf.Face]bool
	usedImages        map[*pdf.Imagefile]bool
	pendingShadings   []pendingShading
	pendingExtGStates []pdf.ExtGState
	pendingSVGForms   []*svgForm
	pendingSVGMasks   []*svgMask
	pendingForms      map[pdf.Name]*pdf.Form
	text              textState
	currentTmY        bag.ScaledPoint
	currentTmYValid   bool
	currentSlant      float64
	textTransparency  float64
	textmode          TextScope
	hasNewline        bool
}

func (oc *objectContext) saveGroupState() groupState {
	return groupState{
		s:                 oc.s,
		form:              oc.form,
		usedFaces:         oc.usedFaces,
		usedImages:        oc.usedImages,
		pendingShadings:   oc.pendingShadings,
		pendingExtGStates: oc.pendingExtGStates,
		pendingSVGForms:   oc.pendingSVGForms,
		pendingSVGMasks:   oc.pendingSVGMasks,
		pendingForms:      oc.pendingForms,
		text:              oc.text,
		currentTmY:        oc.currentTmY,
		currentTmYValid:   oc.currentTmYValid,
		currentSlant:      oc.currentSlant,
		textTransparency:  oc.textTransparency,
		textmode:          oc.textmode,
		hasNewline:        oc.hasNewline,
	}
}

func (oc *objectContext) restoreGroupState(st groupState) {
	oc.s = st.s
	oc.form = st.form
	oc.usedFaces = st.usedFaces
	oc.usedImages = st.usedImages
	oc.pendingShadings = st.pendingShadings
	oc.pendingExtGStates = st.pendingExtGStates
	oc.pendingSVGForms = st.pendingSVGForms
	oc.pendingSVGMasks = st.pendingSVGMasks
	oc.pendingForms = st.pendingForms
	oc.text = st.text
	oc.currentTmY = st.currentTmY
	oc.currentTmYValid = st.currentTmYValid
	oc.currentSlant = st.currentSlant
	oc.textTransparency = st.textTransparency
	oc.textmode = st.textmode
	oc.hasNewline = st.hasNewline
}

// groupOpacity returns the Opacity of v if v is painted as a group.
func groupOpacity(v *node.VList) (float64, bool) {
	if v == nil || v.Attributes == nil {
		return 0, false
	}
	opacity, ok := v.Attributes[Opacity].(float64)
	if !ok || opacity >= 1 {
		return 0, false
	}
	return max(opacity, 0), true
}

// paintGroup calls paint, which paints v, into a Form XObject with a
// transparency group if v has an Opacity, and paints the form with that
// opacity. Without one it just calls paint.
//
// The sequence of a structure element that is open around v closes before
// the form and continues in it and after it: the content of the form is a
// content stream of its own, and a marked-content sequence on the page must
// not contain one in the form. The nodes marked PaintLast inside v are
// painted at the end of the form, so they get the opacity too.
func (oc *objectContext) paintGroup(v *node.VList, paint func()) {
	opacity, ok := groupOpacity(v)
	if !ok {
		paint()
		return
	}
	d := oc.p.document
	if !d.Format.AllowsTransparency() {
		if !d.warnedOpacity {
			d.warnedOpacity = true
			bag.Logger.Warn("The PDF format does not allow transparency, painting opacity opaque")
		}
		paint()
		return
	}

	oc.gotoTextMode(ScopePage)
	parent := oc.interruptRun()
	saved := oc.saveGroupState()

	content := d.PDFWriter.NewObject()
	content.SetCompression(d.CompressLevel)
	g := &groupForm{content: content}
	oc.s = content.Data
	oc.form = g
	oc.usedFaces = make(map[*pdf.Face]bool)
	oc.usedImages = make(map[*pdf.Imagefile]bool)
	oc.pendingShadings = nil
	oc.pendingExtGStates = nil
	oc.pendingSVGForms = nil
	oc.pendingSVGMasks = nil
	oc.pendingForms = nil
	oc.hasNewline = true

	start := len(oc.p.deferred)
	// The content of v belongs to the open element, unless v has a marking
	// of its own.
	inner := parent
	if hasOwnMarking(v) {
		inner = nil
	}
	oc.resumeRun(inner)
	paint()
	oc.gotoTextMode(ScopePage)
	if inner != nil {
		oc.newline()
		oc.writef("EMC\n")
	}
	oc.run = nil
	oc.paintDeferredFrom(start)
	oc.gotoTextMode(ScopePage)

	form := d.PDFWriter.AddForm(content, oc.p.formBBox)
	// Not isolated, unlike the group CSS asks for: with the Normal blend
	// mode only, both look the same, and an overprinting color in the group
	// keeps the plates of what lies below it on the page. In an isolated
	// group it would knock them out by the opacity.
	form.Group = &pdf.TransparencyGroup{}
	if err := oc.materializeGroupResources(form); err != nil {
		bag.Logger.Error("write opacity group", "err", err)
	}
	if len(g.elements) > 0 && d.RootStructureElement != nil {
		po := d.newPDFStructureObject()
		po.formElements = g.elements
		form.Dict = pdf.Dict{"StructParents": fmt.Sprintf("%d", po.id)}
		d.pdfStructureObjects = append(d.pdfStructureObjects, po)
	}

	oc.restoreGroupState(saved)
	d.groupFormCounter++
	name := pdf.Name(fmt.Sprintf("FmOp%d", d.groupFormCounter))
	if oc.pendingForms == nil {
		oc.pendingForms = make(map[pdf.Name]*pdf.Form)
	}
	oc.pendingForms[name] = form
	gs := pdf.ExtGState{FillAlpha: &opacity, StrokeAlpha: &opacity}
	oc.pendingExtGStates = append(oc.pendingExtGStates, gs)
	oc.newline()
	oc.writef("q %s gs %s Do Q\n", gs.ResourceName(), name)
	oc.resumeRun(parent)
}

// paintDeferredFrom paints the nodes deferred since start, as endScope does,
// but also while the sequence of an enclosing element is open on the page:
// in a form, nothing is open around them.
func (oc *objectContext) paintDeferredFrom(start int) {
	d := oc.p.document
	savedTag, savedInArtifact, savedPrefix := oc.tag, oc.inArtifact, d.readingPrefix
	for len(oc.p.deferred) > start {
		oc.gotoTextMode(ScopePage)
		df := oc.p.deferred[start]
		oc.p.deferred = slices.Delete(oc.p.deferred, start, start+1)
		oc.paintDeferred(df)
	}
	oc.tag, oc.run, oc.inArtifact, d.readingPrefix = savedTag, nil, savedInArtifact, savedPrefix
}

// materializeGroupResources gives form the resources its content refers to.
// A form has no access to the resources of its page, so the graphics states
// that every page has (overprinting, the alpha values of colors) go into it
// as well.
func (oc *objectContext) materializeGroupResources(form *pdf.Form) error {
	d := oc.p.document
	for _, f := range slices.SortedFunc(maps.Keys(oc.usedFaces), func(a, b *pdf.Face) int {
		return a.FaceID - b.FaceID
	}) {
		form.Faces = append(form.Faces, f)
	}
	for img := range oc.usedImages {
		form.Images = append(form.Images, img)
		d.placedImages[img] = true
	}
	if err := materializeShadings(&form.Resources, d, oc.pendingShadings); err != nil {
		return err
	}
	gstates := append(d.pageExtGStates(), oc.pendingExtGStates...)
	if err := materializeExtGStates(&form.Resources, d, gstates); err != nil {
		return err
	}
	if err := materializeSVGForms(&form.Resources, d, oc.pendingSVGForms); err != nil {
		return err
	}
	if err := materializeSVGMasks(&form.Resources, d, oc.pendingSVGMasks); err != nil {
		return err
	}
	addForms(&form.Resources, oc.pendingForms)
	return nil
}

// pageExtGStates are the graphics states that every page and every opacity
// group has: a color carries its graphics state in the operators it writes,
// so nothing tells which content stream uses it. Spot colors work the same
// way.
func (d *PDFDocument) pageExtGStates() []pdf.ExtGState {
	var states []pdf.ExtGState
	if d.Overprint {
		states = append(states, pdf.OverprintOn, pdf.OverprintOff)
	}
	for _, name := range slices.Sorted(maps.Keys(d.colorStates)) {
		states = append(states, d.colorStates[name])
	}
	return states
}

// addForms hooks the opacity groups painted in a content stream into its
// resources.
func addForms(res *pdf.Resources, forms map[pdf.Name]*pdf.Form) {
	if len(forms) == 0 {
		return
	}
	if res.Forms == nil {
		res.Forms = make(map[pdf.Name]*pdf.Form)
	}
	maps.Copy(res.Forms, forms)
}
