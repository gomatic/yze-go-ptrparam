// Package nosibling names the same library type as package sibling and imports
// none of the library's other packages. The two exist as a PAIR: they differ
// only in the import list, so a verdict that moved with the imports would make
// them differ, and the analyzer's stated reason — a foreign type follows its
// library's design — is a claim about the type and its library rather than
// about who imports what.
package nosibling

import "graph/ast"

// takesDoc is FLAGGED, exactly as its twin in package sibling is. graph/ast
// mentions *Doc nowhere, so the library publishes no pointer convention for it
// and the verdict is the same whether or not a package that does is imported.
func takesDoc(d *ast.Doc) { _ = d } // want `pointer parameter`
