// Package fabric declares a type whose pointer convention is established
// elsewhere (package weaver) — its own API never mentions *Cloth.
package fabric

// Cloth is the cross-package convention subject.
type Cloth struct{ c int }
