package c

import (
	"fabric"
	"weaver"
)

// takesCloth is FLAGGED although the imported weaver package hands out
// *fabric.Cloth: the convention is read from fabric, the type's own package,
// and fabric's API never mentions *Cloth. Reading it from the analyzed
// package's imports instead made the verdict a function of this file's import
// list — one blank import silenced the finding and removing one restored it.
func takesCloth(cl *fabric.Cloth) { _ = cl } // want `pointer parameter`

// use anchors the weaver import so its API IS visible to the pass, which is
// what makes the case discriminate: the analyzer can see the pointer and
// declines to read it from there.
func use() { _ = weaver.Weave() }
