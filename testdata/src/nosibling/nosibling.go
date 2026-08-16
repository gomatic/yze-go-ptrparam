// Package nosibling names the same library type and imports none of the
// library's other packages.
package nosibling

import "graph/ast"

// takesDoc is FLAGGED, and this case is the residue the exemption still
// carries rather than a property anybody wants: the convention graph/format
// publishes is the same convention whether or not this package imports it, and
// the scan can only read what the pass was handed. Closing it needs the type's
// own module enumerated, which belongs to the framework. It is cased so the
// residue is a case somebody can point at rather than a surprise.
func takesDoc(d *ast.Doc) { _ = d } // want `pointer parameter`
