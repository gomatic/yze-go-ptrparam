// Package weaver hands out fabric.Cloth by pointer: the convention source for
// a type it does not declare.
package weaver

import "fabric"

// Weave returns *fabric.Cloth — the convention consumers follow.
func Weave() *fabric.Cloth { return nil }
