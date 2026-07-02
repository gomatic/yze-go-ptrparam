package c

import (
	"fabric"
	"weaver"
)

// takesCloth is allowed: fabric's own API never mentions *Cloth, but the
// imported weaver package hands it out by pointer.
func takesCloth(cl *fabric.Cloth) { _ = cl }

// use anchors the weaver import so its API is visible to the pass.
func use() { _ = weaver.Weave() }
