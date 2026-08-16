// Package graph is the library's ROOT package, which is where a Go library
// usually puts its entry point: gqlparser's own root hands out
// *ast.QueryDocument, and so does this one. Its import path IS the space its
// ast package hangs off, so the scan reaches it by equality rather than by
// prefix — a different branch of the same test, and the one the archetype
// actually needs.
package graph

import "graph/ast"

// Load hands out the pointer from the library's root.
func Load() *ast.Doc { return nil }
