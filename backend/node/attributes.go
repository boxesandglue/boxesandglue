package node

func getAttribute(a H, attr string) (any, bool) {
	if a == nil {
		return nil, false
	}
	return a[attr], true
}

func setAttribute(a H, attr string, value any) H {
	if a == nil {
		a = H{}
	}
	a[attr] = value
	return a
}

// GetAttribute returns the value of the attribute attr and true or nil and
// false if the attribute does not exist.
func GetAttribute(n Node, attr string) (any, bool) {
	switch t := n.(type) {
	case *Disc:
		return getAttribute(t.Attributes, attr)
	case *Glue:
		return getAttribute(t.Attributes, attr)
	case *Glyph:
		return getAttribute(t.Attributes, attr)
	case *HList:
		return getAttribute(t.Attributes, attr)
	case *Image:
		return getAttribute(t.Attributes, attr)
	case *Kern:
		return getAttribute(t.Attributes, attr)
	case *Lang:
		return getAttribute(t.Attributes, attr)
	case *Penalty:
		return getAttribute(t.Attributes, attr)
	case *Rule:
		return getAttribute(t.Attributes, attr)
	case *StartStop:
		return getAttribute(t.Attributes, attr)
	case *VList:
		return getAttribute(t.Attributes, attr)
	}
	return nil, false
}

// SetAttribute sets the attribute attr on the node n.
func SetAttribute(n Node, attr string, val any) {
	switch t := n.(type) {
	case *Disc:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *Glue:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *Glyph:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *HList:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *Image:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *Kern:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *Lang:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *Penalty:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *Rule:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *StartStop:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	case *VList:
		t.Attributes = setAttribute(t.Attributes, attr, val)
	}
}
