package document

// VTrace determines the type of visual tracing
type VTrace int

const (
	// VTraceImages shows bounding box of images
	VTraceImages VTrace = iota
	// VTraceVBoxes shows the bounding box of vlists
	VTraceVBoxes
	// VTraceHBoxes shows the bounding box of hlists
	VTraceHBoxes
	// VTraceHyperlinks shows hyperlinks
	VTraceHyperlinks
	// VTraceDest shows destinations
	VTraceDest
)

// SetVTrace sets the visual tracing
func (d *PDFDocument) SetVTrace(t VTrace) {
	d.tracing |= 1 << t
}

// ClearVTrace removes the visual tracing.
func (d *PDFDocument) ClearVTrace(t VTrace) {
	d.tracing &^= (1 << t)
}

// IsTrace returns true if tracing t is set
func (d *PDFDocument) IsTrace(t VTrace) bool {
	return (d.tracing>>t)&1 == 1
}
