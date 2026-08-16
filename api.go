package ptrparam

// Reading one package's exported API for mentions of a pointer. This is the
// half of the foreign-convention decision that reads rather than decides:
// foreign.go says WHICH package may speak for a type, and everything here
// answers WHETHER that package's exported surface hands the pointer out. The
// two are separated because the question "does this API mention *T" is a plain
// property of a package and carries none of the exemption's judgement.
//
// The comment sits below the package clause on purpose: a second block above
// it becomes a second package doc, and `go doc .` already prints three of
// those before the rule statement (ptrparam.doc-comment-is-the-one-a-reader-gets,
// k1n81tpf).

import "go/types"

// apiUsesPointer reports whether pkg's exported API mentions *named in a
// parameter, result, method, or exported struct field (directly, or one
// container level deep — []*T, map[...]*T).
//
// It is reached only for a complete pkg; foreignConvention holds that line
// and says why.
func apiUsesPointer(pkg *types.Package, named *types.Named) bool {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj.Exported() && objectUsesPointer(obj, named) {
			return true
		}
	}
	return false
}

// objectUsesPointer inspects one exported package-scope object.
func objectUsesPointer(obj types.Object, named *types.Named) bool {
	switch o := obj.(type) {
	case *types.Func:
		return signatureUsesPointer(o.Type().(*types.Signature), named)
	case *types.TypeName:
		return typeUsesPointer(o, named)
	default:
		return false
	}
}

// typeUsesPointer inspects an exported type's declared methods, interface
// methods, and exported struct fields.
func typeUsesPointer(obj *types.TypeName, named *types.Named) bool {
	switch t := obj.Type().(type) {
	case *types.Named:
		return namedUsesPointer(t, named)
	default:
		return false
	}
}

// namedUsesPointer inspects a named type's method set and then its underlying:
// an interface's methods, a struct's exported fields, or — for a package-level
// FUNC type, which is how a library publishes a callback shape it imposes on
// every caller (grpc's UnaryServerInterceptor, urfave/cli's ActionFunc) — the
// signature itself. Omitting the last one reported the callback parameter of
// every framework that names its own callback type, which is a signature no
// consumer can write any other way.
func namedUsesPointer(t, named *types.Named) bool {
	for i := range t.NumMethods() {
		m := t.Method(i)
		if m.Exported() && signatureUsesPointer(m.Type().(*types.Signature), named) {
			return true
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Interface:
		return interfaceUsesPointer(u, named)
	case *types.Struct:
		return fieldsUsePointer(u, named)
	case *types.Signature:
		return signatureUsesPointer(u, named)
	default:
		return false
	}
}

// interfaceUsesPointer inspects an interface's explicit methods.
func interfaceUsesPointer(iface *types.Interface, named *types.Named) bool {
	for i := range iface.NumExplicitMethods() {
		m := iface.ExplicitMethod(i)
		if m.Exported() && signatureUsesPointer(m.Type().(*types.Signature), named) {
			return true
		}
	}
	return false
}

// fieldsUsePointer inspects a struct's exported fields.
func fieldsUsePointer(st *types.Struct, named *types.Named) bool {
	for f := range st.Fields() {
		if f.Exported() && fieldUsesPointer(f.Type(), named) {
			return true
		}
	}
	return false
}

// fieldUsesPointer inspects one field's type, directly and as a callback.
func fieldUsesPointer(t types.Type, named *types.Named) bool {
	if mentionsPointer(t, named) {
		return true
	}
	sig, ok := t.Underlying().(*types.Signature)
	return ok && signatureUsesPointer(sig, named)
}

// signatureUsesPointer inspects a signature's parameters and results.
func signatureUsesPointer(sig *types.Signature, named *types.Named) bool {
	return tupleMentionsPointer(sig.Params(), named) || tupleMentionsPointer(sig.Results(), named)
}

func tupleMentionsPointer(tuple *types.Tuple, named *types.Named) bool {
	for i := range tuple.Len() {
		if mentionsPointer(tuple.At(i).Type(), named) {
			return true
		}
	}
	return false
}

// mentionsPointer reports whether t is a pointer to named — however the
// library spells that pointer — or a slice/array/map holding one, one level
// deep. The type is compared with types.Identical rather than by generic
// ORIGIN: a library that hands out one instantiation of a generic type has
// established the convention for that instantiation and for no other, and
// keying on the origin exempts every instantiation the library never mentions.
func mentionsPointer(t types.Type, named *types.Named) bool {
	if ptr, ok := pointerType(t); ok {
		return types.Identical(types.Unalias(ptr.Elem()), named)
	}
	switch u := types.Unalias(t).(type) {
	case *types.Slice:
		return mentionsPointer(u.Elem(), named)
	case *types.Array:
		return mentionsPointer(u.Elem(), named)
	case *types.Map:
		return mentionsPointer(u.Elem(), named)
	default:
		return false
	}
}
