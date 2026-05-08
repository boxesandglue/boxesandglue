package node

import (
	"github.com/boxesandglue/boxesandglue/backend/lang"
)

// A Lang is a node that sets the current language.
type Lang struct {
	basenode
	Lang *lang.Lang // The language setting  for the following nodes.
}

func (l *Lang) String() string {
	return "lang: " + l.Lang.Name
}

// DebugAttributes returns the language name (or "-" if unset).
func (l *Lang) DebugAttributes() ([]kv, H) {
	langname := "-"
	if l.Lang != nil {
		langname = l.Lang.Name
	}
	return []kv{
		{key: "id", value: l.ID},
		{key: "lang", value: langname},
	}, l.Attributes
}

// Copy creates a deep copy of the node.
func (l *Lang) Copy() Node {
	n := NewLang()
	n.Lang = l.Lang
	return n
}

// NewLang creates an initialized Lang node
func NewLang() *Lang {
	n := langSlab.alloc()
	n.ID = newID()
	n.typ = TypeLang
	return n
}

// NewLangWithContents creates an initialized Lang node with the given contents
func NewLangWithContents(l *Lang) *Lang {
	n := langSlab.alloc()
	*n = *l
	n.ID = newID()
	n.typ = TypeLang
	return n
}
