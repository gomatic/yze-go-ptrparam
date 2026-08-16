// Package rootpub names the library's type and imports the library's ROOT
// package rather than one of its subdirectories.
package rootpub

import (
	"graph"
	"graph/ast"
)

// takesDoc is allowed: graph.Load hands out *ast.Doc, and `graph` is the space
// `graph/ast` hangs off rather than a package under it. A prefix test written
// without the equality arm reaches every sibling and misses the root, which is
// the one the doc names first.
func takesDoc(d *ast.Doc) { _ = d }

// use anchors the import without adding a subject of its own.
func use() { _ = graph.Load() }
