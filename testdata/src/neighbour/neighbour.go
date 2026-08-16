// Package neighbour is the boundary case: the publishing package's path shares
// the library's space as a text prefix and is not inside it.
package neighbour

import (
	"graph/ast"
	"graphx/pub"
)

// takesDoc is FLAGGED: `graphx/pub` is not in `graph`, and a prefix test
// written without the separator would read it as though it were — which would
// hand every neighbouring path space the power to exempt a library's types.
func takesDoc(d *ast.Doc) { _ = d } // want `pointer parameter`

// use anchors the import so its API IS visible to the pass, which is what makes
// the case discriminate.
func use() { _ = pub.Hand() }
