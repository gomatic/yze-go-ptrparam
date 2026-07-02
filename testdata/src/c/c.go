// Package c consumes a foreign library; foreign types follow the library's
// convention, module-local judgments do not apply to them.
package c

import "lib"

// takesNode is allowed: lib's exported API returns *Node, so the pointer is
// the library's convention for it.
func takesNode(n *lib.Node) { _ = n }

// takesOptions is flagged: nothing in lib's API hands out or accepts
// *Options, so passing it by pointer is this module's own choice.
func takesOptions(o *lib.Options) { _ = o } // want `pointer parameter`

// takesCtx is allowed: lib.Walker's interface method accepts *Ctx.
func takesCtx(c *lib.Ctx) { _ = c }

// takesLeaf is allowed: lib.Tree exports a *Leaf field.
func takesLeaf(l *lib.Leaf) { _ = l }

// takesItem is allowed: lib.Handler's callback field receives []*Item.
func takesItem(i *lib.Item) { _ = i }

// takesEntry is allowed: lib.Lookup returns map[string]*Entry.
func takesEntry(e *lib.Entry) { _ = e }

// takesRow is allowed: lib.Grid carries [3]*Row.
func takesRow(r *lib.Row) { _ = r }

// takesWidget is allowed: lib.Factory.New returns *Widget.
func takesWidget(w *lib.Widget) { _ = w }
