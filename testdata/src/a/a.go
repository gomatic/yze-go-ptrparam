package a

import (
	"bytes"
	"log/slog"
)

type Plain struct{ x int }

// BufAlias is an alias of an allow-listed standard-library type.
type BufAlias = bytes.Buffer

// takesLocal is flagged: pointer to a local type.
func takesLocal(p *Plain) { _ = p } // want `pointer parameter`

// takesInt is flagged: pointer to a basic type.
func takesInt(n *int) { _ = n } // want `pointer parameter`

// takesErr is flagged: pointer to error (named, no package).
func takesErr(e *error) { _ = e } // want `pointer parameter`

// takesLogger is allowed: a standard-library type where a pointer is idiomatic.
func takesLogger(l *slog.Logger) { _ = l }

// takesValue is fine: a value parameter.
func takesValue(p Plain) { _ = p }

// aliasOK is allowed: a pointer to an alias of an allow-listed stdlib type.
func aliasOK(b *BufAlias) { _ = b }

// variadicPtr is flagged: a variadic pointer parameter.
func variadicPtr(xs ...*int) { _ = xs } // want `pointer parameter`
