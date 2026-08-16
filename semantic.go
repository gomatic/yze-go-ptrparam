// Semantic pointer-idiomatic detection: the discovery criteria applied live to
// any package's types.
package ptrparam

import (
	"go/token"
	"go/types"
)

// nullary is the signature `func()`, the shape both sync.Locker methods have.
func nullary() *types.Signature { return types.NewSignatureType(nil, nil, nil, nil, nil, false) }

// lockerType is sync.Locker — Lock() and Unlock(), both nullary. It is built
// here the way copylock.go builds it, so this package and go vet agree on the
// must-not-copy marker by construction rather than by resemblance.
var lockerType = types.NewInterfaceType([]*types.Func{
	types.NewFunc(token.NoPos, nil, "Lock", nullary()),
	types.NewFunc(token.NoPos, nil, "Unlock", nullary()),
}, nil).Complete()

// pointerIdiomatic decides, from the type itself, whether a pointer is its
// idiomatic calling convention — the same criteria the analyzed module's own
// types are held to, applied to ANY package (a framework's *analysis.Pass, a
// parser library's AST nodes, an uncopyable local type):
//
//   - uncopyable: the type transitively holds a lock (vet's copylocks
//     criterion), so copying corrupts it;
//   - pointer-only methods: every exported declared method takes the pointer
//     receiver, so a value carries no usable API.
//
// A repo's own value types stay flagged: ptrrecv drives their receivers to
// values, and a type whose receivers legitimately stay pointers (it mutates)
// is legitimately passed by pointer.
// The criterion is applied to the NAMED type, not to its underlying: the
// marker lives in the method set, which an underlying struct does not carry.
// Judging the underlying alone reported a parameter whose type go vet reports
// every copy of, which is a diagnostic prescribing a remedy the standard
// toolchain then rejects.
func pointerIdiomatic(named *types.Named) bool {
	return uncopyable(named) || pointerOnlyMethods(named.Origin())
}

// pointerOnlyMethods reports whether the type declares exported methods and
// every one of them takes the pointer receiver.
//
// This exemption is forgeable by design and its forgery is charged elsewhere:
// exporting a pointer-receiver method IS the Go-idiomatic way to declare that
// a type is handled by pointer, so an author who writes one has said the thing
// the exemption exists for, and yze/ptrrecv reports the receiver they wrote.
// The marker cannot be acquired without acquiring the property or the finding.
func pointerOnlyMethods(named *types.Named) bool {
	exported := 0
	for i := range named.NumMethods() {
		m := named.Method(i)
		if !m.Exported() {
			continue
		}
		exported++
		if _, isPtr := m.Type().(*types.Signature).Recv().Type().(*types.Pointer); !isPtr {
			return false
		}
	}
	return exported > 0
}

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
func uncopyableWithin(seen visited, t types.Type) bool {
	if !seen.entering(t) {
		return false
	}
	if param, isParam := types.Unalias(t).(*types.TypeParam); isParam {
		return constraintUncopyable(seen, param)
	}
	return mustNotCopy(t) || componentUncopyable(seen, t)
}

// constraintUncopyable applies the criterion to every type a type parameter's
// constraint admits, which is what go vet's copylocks does through its own
// type-parameter arm. Without it a generic struct holding a lock-constrained
// field was reported and the prescribed value parameter drew
// `passes lock by value: G[T] contains T contains sync.Mutex` from go vet —
// two tools in one suite issuing contradictory instructions about one line.
// A constraint's underlying is an interface in every form the language
// admits, because go/types wraps a non-interface one implicitly — `[T int]`
// arrives here as an interface holding `int` as its single element, which
// TestConstraintUncopyableReadsEveryConstraintFormGoTypesProduces asserts — so
// the assertion below carries no branch for a case that cannot occur.
func constraintUncopyable(seen visited, param *types.TypeParam) bool {
	iface := param.Constraint().Underlying().(*types.Interface)
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

// componentUncopyable descends into struct fields and array elements.
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
	}
	return false
}

// mustNotCopy applies go vet's copylocks criterion, which semantic.go's own
// doc cites: a STRUCT whose POINTER is a sync.Locker while its value is not.
//
// Requiring only a nullary Lock — this package's earlier test — is strictly
// wider than the standard it cites. `type Locky struct{ X int }` with a single
// `func (l *Locky) Lock()` copies with go vet completely silent about it, so
// one method named Lock exempted a type and, through componentUncopyable,
// every struct that held it at any depth: two lines of forgery, drawing
// nothing from any of the 42 analyzers, for a type `func Copy(s T) T` still
// copies correctly. Under vet's marker the forgery costs both methods on the
// pointer receiver, which is the point at which go vet starts reporting every
// copy of the type — acquiring the marker is acquiring the property.
func mustNotCopy(t types.Type) bool {
	if _, isStruct := t.Underlying().(*types.Struct); !isStruct {
		return false
	}
	return types.Implements(types.NewPointer(t), lockerType) && !types.Implements(t, lockerType)
}
