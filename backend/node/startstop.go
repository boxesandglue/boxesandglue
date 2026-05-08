package node

import "fmt"

// PDFDataOutput defines the location of inserted PDF data.
type PDFDataOutput int

// ActionType represents a start/stop action such as a PDF link.
type ActionType int

const (
	// PDFOutputNone ignores any movement commands.
	PDFOutputNone PDFDataOutput = iota
	// PDFOutputHere inserts ET and moves to current position before inserting
	// the PDF data.
	PDFOutputHere
	// PDFOutputDirect inserts the PDF data without leaving the text mode with ET.
	PDFOutputDirect
	// PDFOutputPage inserts ET before writing the PDF data.
	PDFOutputPage
	// PDFOutputLowerLeft moves to the lower left corner before inserting the
	// PDF data.
	PDFOutputLowerLeft
)

const (
	// ActionNone represents no special action
	ActionNone ActionType = iota
	// ActionHyperlink represents a hyperlink.
	ActionHyperlink
	// ActionDest insets a PDF destination.
	ActionDest
	// ActionUserSetting allows user defined settings.
	ActionUserSetting
)

func (at ActionType) String() string {
	switch at {
	case ActionNone:
		return "ActionNone"
	case ActionHyperlink:
		return "ActionHyperlink"
	case ActionDest:
		return "ActionDest"
	case ActionUserSetting:
		return "ActionUserSetting"
	default:
		return "other action"
	}
}

// StartStopFunc is the type of the callback when this node is encountered in
// the node list. The returned string (if not empty) gets written to the PDF.
//
// ARCHITECTURAL DEBT — known layering violation. The callback returns a raw
// PDF string, baking the output backend into the (otherwise format-neutral)
// node data model: a hypothetical SVG renderer would have no use for a
// closure that hands it `" 1 0 0 rg "`. The hook also conflates two
// distinct mechanisms under one signature:
//
//  1. Code generators. Synthesize per-node PDF instructions on the fly.
//     Today: the color start / stop nodes in frontend/nodebuilding.go
//     (returning things like `col.PDFStringNonStroking() + " "`). A backend-
//     neutral replacement puts the domain payload (e.g. *color.Color) into
//     StartStop.Value with a typed Action constant and lets the renderer
//     in backend/document/document.go format it for the active output.
//
//  2. Side-effect hooks. Fire when shipout reaches the node and return ""
//     (or near-empty). Today: xts marker registration and PDF-outline
//     construction in xts/core/commands.go. These do not want to emit
//     bytes into the page stream — they want "remember the page this
//     landed on" or "register a destination once its position is known".
//     Their natural home is a per-node pre-shipout visitor, parallel to
//     document.preShipoutCallback which already exists for whole-page hooks.
//
// Refactoring is a multi-step change (audit callers, introduce new Action
// constants, migrate the side-effect hooks via a separate mechanism) and
// earns its keep when a second output backend (SVG / HTML / …) becomes a
// concrete requirement, or when StartStop nodes need to be inspectable
// (e.g. accessibility tooling reading the active color without re-parsing
// generated PDF).
type StartStopFunc func(thisnode Node) string

// A StartStop is a paired node type used for color switches, hyperlinks and
// such.
type StartStop struct {
	basenode
	// Value contains action specific contents
	Value           any
	StartNode       *StartStop
	ShipoutCallback StartStopFunc
	Action          ActionType
	Position        PDFDataOutput
}

func (d *StartStop) String() string {
	return String(d)
}

// NewStartStop creates an initialized Start node
func NewStartStop() *StartStop {
	n := startStopSlab.alloc()
	n.ID = newID()
	n.typ = TypeStartStop
	return n
}

// DebugAttributes returns the action plus the start node id (or "-").
func (d *StartStop) DebugAttributes() ([]kv, H) {
	startNode := "-"
	if d.StartNode != nil {
		startNode = fmt.Sprintf("%d", d.StartNode.ID)
	}
	return []kv{
		{key: "id", value: d.ID},
		{key: "action", value: d.Action},
		{key: "start", value: startNode},
	}, d.Attributes
}

// Copy creates a deep copy of the node.
func (d *StartStop) Copy() Node {
	n := NewStartStop()
	n.Action = d.Action
	n.StartNode = d.StartNode
	n.Position = d.Position
	n.ShipoutCallback = d.ShipoutCallback
	n.Value = d.Value
	return n
}
