// Semantic pointer-idiomatic detection: the discovery criteria applied live to
// any package's types.
package ptrparam

import "go/types"

// pointerIdiomatic decides, from the type itself, whether a pointer is its
// idiomatic calling convention — the same criteria the generated stdlib
// allowlist is discovered under, applied to ANY package (a framework's
// *analysis.Pass, a parser library's AST nodes, an uncopyable local type):
//
//   - uncopyable: the type transitively holds a lock (vet's copylocks
//     criterion), so copying corrupts it;
//   - pointer-only methods: every exported declared method takes the pointer
//     receiver, so a value is unusable.
//
// A repo's own value types stay flagged: ptrrecv drives their receivers to
// values, and a type whose receivers legitimately stay pointers (it mutates)
// is legitimately passed by pointer.
func pointerIdiomatic(named *types.Named) bool {
	return uncopyable(named.Underlying(), map[types.Type]bool{}) || pointerOnlyMethods(named.Origin())
}

// pointerOnlyMethods reports whether the type declares exported methods and
// every one of them takes the pointer receiver.
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

// uncopyable reports whether t transitively holds a lock — the copylocks
// criterion: any component type whose pointer method set has a nullary Lock
// method.
func uncopyable(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	if hasPointerLock(t) {
		return true
	}
	return componentUncopyable(t, seen)
}

// componentUncopyable descends into struct fields and array elements.
func componentUncopyable(t types.Type, seen map[types.Type]bool) bool {
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for f := range u.Fields() {
			if uncopyable(f.Type(), seen) {
				return true
			}
		}
	case *types.Array:
		return uncopyable(u.Elem(), seen)
	}
	return false
}

// hasPointerLock reports whether *t has a nullary Lock method — the marker
// vet's copylocks uses for must-not-copy types (sync primitives and noCopy).
func hasPointerLock(t types.Type) bool {
	if _, isPtr := t.Underlying().(*types.Pointer); isPtr {
		return false
	}
	set := types.NewMethodSet(types.NewPointer(t))
	for i := range set.Len() {
		if fn := set.At(i).Obj(); fn.Name() == "Lock" && isNullary(fn) {
			return true
		}
	}
	return false
}

// isNullary reports a func with no parameters and no results — Lock()'s shape.
func isNullary(obj types.Object) bool {
	sig, ok := obj.Type().(*types.Signature)
	return ok && sig.Params().Len() == 0 && sig.Results().Len() == 0
}
