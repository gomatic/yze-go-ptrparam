// Package pub sits at an import path that SHARES A PREFIX with the library's
// space without being inside it: `graphx` against `graph`. It hands out the
// pointer, and it must establish nothing.
package pub

import "graph/ast"

// Hand hands out the pointer from just outside the library's space.
func Hand() *ast.Doc { return nil }
