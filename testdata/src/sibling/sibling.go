// Package sibling names the library's type and imports a package the library
// itself owns.
package sibling

import (
	"graph/ast"
	"graph/format"

	"lib"
)

// takesDoc is allowed: graph/format is inside graph/ast's path space and hands
// the pointer out, so the pointer is the library's convention. The remedy
// prescribed without this — pass the value — silently loses every mutation the
// callee makes through the alias.
func takesDoc(d *ast.Doc) { _ = d }

// takesOptions is flagged: lib is not in graph/ast's space and nothing in it
// hands out *ast.Doc, so importing it establishes nothing. It is here so the
// scan is seen to reject a package as well as accept one.
func takesOptions(o *lib.Options) { _ = o } // want `pointer parameter`

// use anchors both imports without adding a second subject of its own.
func use() { format.Format(&ast.Doc{}) }
