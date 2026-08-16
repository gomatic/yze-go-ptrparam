// Package forge is the forgery: a package the AUTHOR writes, handing out a
// pointer to somebody else's type in order to buy silence for it.
package forge

import "graph/ast"

// Hand hands out *ast.Doc from a package outside the library's path space.
func Hand() *ast.Doc { return nil }
