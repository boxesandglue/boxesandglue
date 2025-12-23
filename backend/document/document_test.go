package document

import "testing"

func TestScopeHierarchy(t *testing.T) {
	// The code relies on the scope constants being ordered from innermost to outermost.
	// This test ensures that invariant is maintained.
	if !(ScopeGlyph < ScopeArray) {
		t.Error("ScopeGlyph must be less than ScopeArray")
	}
	if !(ScopeArray < ScopeText) {
		t.Error("ScopeArray must be less than ScopeText")
	}
	if !(ScopeText < ScopePage) {
		t.Error("ScopeText must be less than ScopePage")
	}
}
