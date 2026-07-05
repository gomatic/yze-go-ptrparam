// Foreign-convention detection: the analyzed module's own types are its design
// responsibility, but a foreign type follows its library's conventions.
package ptrparam

import (
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// foreignConvention reports whether a type from OUTSIDE the analyzed module is
// conventionally handled by pointer: the module's own types are the analyzed
// code's design responsibility, but a foreign type follows its library's
// design — when the library's exported API hands out or accepts *T (a parser
// returning *ast.Document whose nodes are aliased and mutated in place),
// forcing values onto it would be wrong code, not style. The signal is read
// from the type's own package plus the analyzed package's direct imports
// (constructors often live beside the type, not inside its package).
//
// A named type with basic underlying never gains the immunity — the same
// exclusion discover.go's recordPointer applies to the generated allowlist:
// an API taking *T there (flag.DurationVar's *time.Duration) is an
// out-parameter binding to a value-idiomatic type, not a passing convention.
func foreignConvention(pass *analysis.Pass, named *types.Named) bool {
	if _, isBasic := named.Underlying().(*types.Basic); isBasic {
		return false
	}
	pkg := named.Obj().Pkg()
	if localToModule(pass, pkg) {
		return false
	}
	if apiUsesPointer(pkg, named) {
		return true
	}
	for _, imp := range pass.Pkg.Imports() {
		if apiUsesPointer(imp, named) {
			return true
		}
	}
	return false
}

// localToModule reports whether pkg belongs to the analyzed module. Without
// module metadata (a driver that does not load it), only the analyzed package
// itself counts as local — the conservative fallback.
func localToModule(pass *analysis.Pass, pkg *types.Package) bool {
	if pkg == pass.Pkg {
		return true
	}
	if pass.Module == nil || pass.Module.Path == "" {
		return false
	}
	return pkg.Path() == pass.Module.Path || strings.HasPrefix(pkg.Path(), pass.Module.Path+"/")
}

// apiUsesPointer reports whether pkg's exported API mentions *named in a
// parameter, result, method, or exported struct field (directly, or one
// container level deep — []*T, map[...]*T).
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

// namedUsesPointer inspects a named type's method set and struct fields.
func namedUsesPointer(t, named *types.Named) bool {
	for i := range t.NumMethods() {
		m := t.Method(i)
		if m.Exported() && signatureUsesPointer(m.Type().(*types.Signature), named) {
			return true
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok && interfaceUsesPointer(iface, named) {
		return true
	}
	st, ok := t.Underlying().(*types.Struct)
	return ok && fieldsUsePointer(st, named)
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

// mentionsPointer reports whether t is *named, or a slice/array/map holding
// *named one level deep.
func mentionsPointer(t types.Type, named *types.Named) bool {
	switch u := t.(type) {
	case *types.Pointer:
		elem, ok := types.Unalias(u.Elem()).(*types.Named)
		return ok && elem.Origin() == named.Origin()
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
