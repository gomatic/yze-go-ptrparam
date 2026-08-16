package ptrparam

// The transitive copy-hazard walk: whether a lock is reachable from a type
// through its struct fields, array elements, and the type sets of the
// interfaces and constraints it names. semantic.go owns the MARKERS — what
// counts as a lock, what counts as a pointer-only method set — and this file
// owns the DESCENT that looks for one, together with the cycle guard the
// descent cannot be run without.
//
// The comment sits below the package clause on purpose: a block above it
// becomes a second package doc, and `go doc .` already prints several of those
// before the rule statement (ptrparam.doc-comment-is-the-one-a-reader-gets,
// k1n81tpf).

import "go/types"

// visited is the set of types one walk has already descended into. A single
// set is threaded through the whole descent, so re-entering a type means the
// walk has closed a cycle rather than found a second route worth taking.
type visited map[types.Type]bool

// entering records t and reports whether this is the first time the walk has
// descended into it. Answering false on a repeat is the sound answer, not a
// surrender: the only way back to t is the edge the walk is already on, so
// nothing reachable only through it is reachable at all. A lock on any other
// edge is still found, because the sibling edges are still walked.
func (v visited) entering(t types.Type) bool {
	if v[t] {
		return false
	}
	v[t] = true
	return true
}

// uncopyable reports whether t transitively holds a lock — the copylocks
// criterion.
func uncopyable(t types.Type) bool { return uncopyableWithin(visited{}, t) }

// uncopyableWithin is the walk, and the cycle guard is here because every
// recursive edge in this file passes through it.
//
// The guard is the one go vet's copylocks carries for the same walk —
// copylock.go:282-290 in golang.org/x/tools, "The seen map is used to
// short-circuit infinite recursion due to type cycles" — and it is needed for
// the same arm. Descending only through struct fields and array elements would
// not need it, because Go rejects every value-type cycle reachable that way
// ("invalid recursive type", verified for `struct{ a [1]T }`, for a two-type
// A/B cycle, and for a cycle through a generic instantiation). Descending
// through a TYPE PARAMETER's constraint does need it: `[T interface{ ~struct{
// N T } }]` is legal Go that `go build` and `go vet` both accept, and it puts
// this walk on a cycle the language does not reject. Without the guard the
// process died with `fatal error: stack overflow` — not a wrong verdict but no
// verdict at all, for every analyzer sharing the run.
//
// A TYPE PARAMETER NEEDS NO ARM OF ITS OWN, and it had one until this was
// measured. `types.TypeParam.Underlying()` returns its constraint's underlying
// — the SAME `*types.Interface` object, asserted by
// TestATypeParametersUnderlyingIsItsConstraintsInterface for every constraint
// form the language admits — so a dedicated dispatch to the constraint and
// componentUncopyable's interface arm below are the same call on the same
// object. The dispatch was written first and the interface arm landed after it,
// which is the moment it became dead code: no input reaches one and not the
// other, and a mutation deleting it left every probe's report and the whole
// suite unmoved. docs/s03.md calls that inert rather than untested and says to
// remove it, because a case for it reports the same against a sound tree as
// against a broken one and would only make it look tested. What the walk needs
// from a constraint is
// unchanged and still cased: go vet's copylocks resolves a type parameter
// through its constraint, and a diagnostic that skipped the step prescribed a
// value parameter vet then rejected.
func uncopyableWithin(seen visited, t types.Type) bool {
	if !seen.entering(t) {
		return false
	}
	return mustNotCopy(t) || componentUncopyable(seen, t)
}

// interfaceUncopyable walks every element of an interface's type set. It is
// reached both from a constraint and from componentUncopyable, because a
// constraint element may itself be an interface — `interface{ Locked }` where
// `Locked` is `interface{ sync.Mutex }` is the same type set one embedding
// away, and stopping at the embedding judged it copyable while go vet reported
// every copy of it.
func interfaceUncopyable(seen visited, iface *types.Interface) bool {
	for i := range iface.NumEmbeddeds() {
		if embeddedUncopyable(seen, iface.EmbeddedType(i)) {
			return true
		}
	}
	return false
}

// embeddedUncopyable walks one constraint element: a union of terms, or a
// single type standing for itself.
func embeddedUncopyable(seen visited, t types.Type) bool {
	union, isUnion := t.(*types.Union)
	if !isUnion {
		return uncopyableWithin(seen, t)
	}
	for i := range union.Len() {
		if uncopyableWithin(seen, union.Term(i).Type()) {
			return true
		}
	}
	return false
}

// componentUncopyable descends into struct fields, array elements, and the
// elements of an interface's type set. The last is what an embedded constraint
// arrives as, and it is safe to descend into only because the walk carries a
// cycle guard: an interface embedding chain can close on itself, which a struct
// or an array cannot.
//
// It is also the whole of the type-parameter arm. `Underlying()` of a
// `*types.TypeParam` is its constraint's interface, so a type parameter reaches
// the interface case here and its constraint's type set is walked with no
// dispatch above deciding anything.
func componentUncopyable(seen visited, t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for f := range u.Fields() {
			if uncopyableWithin(seen, f.Type()) {
				return true
			}
		}
	case *types.Array:
		return uncopyableWithin(seen, u.Elem())
	case *types.Interface:
		return interfaceUncopyable(seen, u)
	}
	return false
}
