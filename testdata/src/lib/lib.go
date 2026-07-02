// Package lib is a stand-in foreign library: Node is handed out by pointer
// from its exported API (the parser-AST shape); Options has no pointer API.
package lib

// Node is an AST-like node the library aliases and mutates in place.
type Node struct{ Kids []*Node }

// Parse returns the pointer — the library's convention.
func Parse() *Node { return &Node{} }

// Options is plain configuration; nothing in the API hands out *Options.
type Options struct{ N int }

// takesSpecialShape is unexported: it must NOT establish a convention.
func takesSpecialShape(o *Options) { _ = o }

// Ctx is conventioned only through an interface method.
type Ctx struct{ c int }

// Walker mentions *Ctx in an interface method.
type Walker interface{ Walk(c *Ctx) }

// Leaf is conventioned only through an exported struct field.
type Leaf struct{ n int }

// Tree carries *Leaf in an exported field.
type Tree struct{ First *Leaf }

// Item is conventioned through a func-typed field's slice parameter.
type Item struct{ v int }

// Handler carries a callback receiving []*Item.
type Handler struct{ OnItems func(items []*Item) }

// Entry is conventioned through a map-valued result.
type Entry struct{ e int }

// Lookup returns *Entry values one map level deep.
func Lookup() map[string]*Entry { return nil }

// Row is conventioned through an array field.
type Row struct{ r int }

// Grid carries rows by pointer in an array.
type Grid struct{ Rows [3]*Row }

// Default is a package var: vars establish no convention.
var Default *Options

// Str is an alias TypeName whose type is not named.
type Str = string

// Widget is conventioned through a concrete type's method result.
type Widget struct{ w int }

// Factory hands out *Widget from a method.
type Factory struct{}

// New returns the pointer — convention via method signature.
func (Factory) New() *Widget { return &Widget{} }
