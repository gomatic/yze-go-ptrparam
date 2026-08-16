// Package forged is the forgery attempt: it names the library's type and
// imports a package handing out the pointer that the AUTHOR wrote themselves.
package forged

import (
	"forge"
	"graph/ast"
)

// takesDoc is FLAGGED although an imported package hands out *ast.Doc: `forge`
// is not in the space graph/ast hangs off, so it establishes no convention for
// anything in it. Two lines and one import line used to be the whole cost of
// silencing a foreign type — a marker acquiring none of the property it stands
// for, and the reason the unrestricted scan was deleted.
func takesDoc(d *ast.Doc) { _ = d } // want `pointer parameter`

// use anchors the forge import so its API IS visible to the pass, which is
// what makes the case discriminate: the analyzer can see the pointer and
// declines to read it from there.
func use() { _ = forge.Hand() }
