// Package sibling names a library's type and imports a package sitting inside
// that library's own import space — the strongest form of the marker the
// deleted sibling scan read, and the forgery case for it.
package sibling

import (
	"graph/ast"
	"graph/format"

	"lib"
)

// takesDoc is FLAGGED although `graph/format` is inside `graph/ast`'s import
// space and hands the pointer out. That is the forgery case: an import path is
// a claim the analyzed build makes, not one anybody published. Three files —
// a go.mod naming the path, a one-line function taking *ast.Doc, and a
// `replace` directive — put a package at any path in the author's own tree, so
// the marker was acquired without acquiring one word of the property it stands
// for. The library still hands out no pointer and passing the value is still
// correct code.
func takesDoc(d *ast.Doc) { _ = d } // want `pointer parameter`

// takesOptions is FLAGGED for the same reason from the other direction: `lib`
// is outside `graph/ast`'s space and hands out no *ast.Doc. The pair is here so
// the case reads as one rule rather than as a space test — after the scan's
// deletion the import space decides nothing, and reinstating any version of it
// turns takesDoc silent while leaving takesOptions alone.
func takesOptions(o *lib.Options) { _ = o } // want `pointer parameter`

// use anchors both imports so their APIs ARE materialised for the pass, which
// is what makes the case discriminate: the analyzer can see the pointer
// graph/format hands out and declines to read it.
func use() { format.Format(&ast.Doc{}) }
