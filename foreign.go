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
// forcing values onto it would be wrong code, not style.
//
// The signal is read from the type's OWN package and from nowhere else. It
// used to be read from the analyzed package's direct imports as well, on the
// reasoning that constructors often live beside the type rather than inside
// its package, and that made the verdict a function of the analyzed file's
// import list: adding `_ "weaver"` to a file, one line that changes nothing
// else and reads as ordinary housekeeping, turned a reported parameter silent,
// and deleting an import a refactor no longer needed turned a green file red
// for a reason nothing in the diff explained. A decision about a type must
// read something the judging file cannot rewrite.
//
// Where the loader did not materialise the library, the analyzer renders no
// verdict on it. go/types populates a package's scope only as far as the
// loader needed it, so a type reached through another package's alias
// re-export arrives with its own package holding one name and nothing else;
// scanning that scope answers "no convention" for a library whose convention
// was merely not loaded. An unreadable library is not a library with no
// pointer convention.
//
// THIS IS A DISABLEMENT CHANNEL, not merely a limitation, and it is recorded
// as one rather than described as a virtue. An author who writes a two-line
// alias package in their own module and names the type through it keeps the
// type's own package out of the load, and the parameter goes silent — one
// blank import of that package puts it back. Neither polarity is
// import-independent, because blindness cannot tell "this library publishes no
// pointer convention" from "this library was not loaded"; this one was chosen
// because it costs a new package a reviewer can see rather than one invisible
// import line. The repair is not a polarity: it is a loader that materialises
// the package, which belongs to the framework.
//
// A named type with basic underlying never gains the immunity: an API taking
// *T there (flag.DurationVar's *time.Duration) is an out-parameter binding to
// a value-idiomatic type, not a passing convention.
func foreignConvention(pass *analysis.Pass, named *types.Named) bool {
	if _, isBasic := named.Underlying().(*types.Basic); isBasic {
		return false
	}
	pkg := named.Obj().Pkg()
	if localToModule(pass, pkg) {
		return false
	}
	if !pkg.Complete() {
		return true
	}
	return apiUsesPointer(pkg, named)
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
