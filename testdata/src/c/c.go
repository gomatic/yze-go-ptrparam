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

// takesHook is allowed: lib.OnHook is a package-level func type receiving
// *Hook, which is the shape the library imposes on every implementation of
// it. A consumer cannot write that signature any other way.
func takesHook(h *lib.Hook) { _ = h }

// takesHidden is flagged: lib mentions *Hidden only from lib.Vault's
// UNEXPORTED field. A convention a consumer cannot see is not a convention,
// and reading unexported fields would exempt a type on evidence no caller has.
func takesHidden(h *lib.Hidden) { _ = h } // want `pointer parameter`

// takesIntBox is allowed: lib.MakeIntBox hands out *Box[int].
func takesIntBox(b *lib.Box[int]) { _ = b }

// takesStringBox is flagged: the exemption follows what the library actually
// hands out, and nothing in lib produces or accepts *Box[string]. Following
// the generic's NAME instead would exempt every instantiation of any generic
// the library mentions once.
func takesStringBox(b *lib.Box[string]) { _ = b } // want `pointer parameter`

// takesCursor is allowed: lib.Open hands out Handle, the library's own
// defined-pointer spelling for *Cursor. The convention scan reads the type,
// not the spelling, exactly as the parameter side does.
func takesCursor(c *lib.Cursor) { _ = c }

// takesHandle is allowed: the same pointer, spelled as the library spells it.
func takesHandle(h lib.Handle) { _ = h }

// localNode is DEFINED over a foreign type whose only exemption is its
// library's published convention: lib.Parse hands out *lib.Node, and lib.Node
// declares no method and holds no lock. Flagged. The definition inherits
// lib.Node's LAYOUT, which carries no copy hazard, and it cannot inherit the
// convention, because lib's API mentions localNode nowhere — no signature in
// the library can be handed one, so passing it by value is code lib never
// sees. Inheriting the convention made this one `type` line the cheapest
// silence in the analyzer.
type localNode lib.Node

func takesLocalNode(n *localNode) { _ = n } // want `pointer parameter`

// localHandled is one link further along the same chain, so a fix that stopped
// at the first link would still exempt it.
type localHandled localNode

func takesLocalHandled(h *localHandled) { _ = h } // want `pointer parameter`
