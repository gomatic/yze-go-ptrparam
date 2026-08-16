// Package format is the library's sibling: it hands out the pointer its own
// ast package never mentions. An author cannot write a package at this import
// path without owning the library's path space, which is what makes it a
// marker the judged code cannot forge.
package format

import "graph/ast"

// Format takes the pointer — the library's convention for ast.Doc.
func Format(d *ast.Doc) { _ = d }
