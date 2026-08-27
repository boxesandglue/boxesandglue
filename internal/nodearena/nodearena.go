// Package nodearena connects the node package's chunk allocator to the
// document lifecycle. It lives in internal/ on purpose: releasing allocator
// chunks is an implementation detail that should not become public API
// (issue #21). The node package registers its release function here at init
// time; document.Finish calls Release.
package nodearena

var release func()

// SetRelease registers the function that drops the node allocator's
// retained chunks. Called once from the node package's init.
func SetRelease(f func()) {
	release = f
}

// Release drops the node allocator's retained chunks. Nodes that are still
// referenced remain valid; only memory that would otherwise stay pinned by
// the allocator is handed back to the garbage collector. Safe to call at any
// time, also while other goroutines allocate nodes.
func Release() {
	if release != nil {
		release()
	}
}
