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
