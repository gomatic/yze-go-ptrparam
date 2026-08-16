// Foreign-convention detection: the analyzed module's own types are its design
// responsibility, but a foreign type follows its library's conventions.
package ptrparam

import (
	"go/types"
	"path"
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
// The signal is read from the type's own package, and from the imported
// packages THE LIBRARY ITSELF OWNS — those whose import path sits in the space
// the type's package hangs off. It used to be read from every direct import,
// on the reasoning that constructors often live beside the type rather than
// inside its package. The reasoning is right and the scan was too wide: it
// made the verdict a function of the analyzed file's import list, because ANY
// package could be the one that hands the pointer out. Adding `_ "weaver"` to
// a file — one line that changes nothing else and reads as ordinary
// housekeeping — turned a reported parameter silent, and the `weaver` it named
// was two lines the author wrote themselves.
//
// Reading only the type's own package answers that and costs the shape the
// exemption was written for first. Go libraries put types in `ast/` and the
// operations on them beside it: `github.com/vektah/gqlparser/v2/ast` mentions
// `*QueryDocument` nowhere, and `parser`, `validator`, `formatter` and the
// root package all hand it out or take it. Measured, that silence added 21
// findings on one type across the fleet, and the remedy prescribed for them
// was built and RUN: passing the value loses the appended operation silently,
// with go vet and this analyzer both exiting 0 on the remedied source. A
// diagnostic whose remedy corrupts leaves the author the baseline, which is
// the population this rule is meant to shrink.
//
// The path space is what discriminates the two. `weaver` is not in `fabric`'s
// space and never becomes a convention source for it; `formatter` is in
// `ast`'s, and an author cannot write a package there without owning the
// library's path. So the marker stays out of reach of the code being judged:
// forging it means publishing inside somebody else's import path, and a
// library that already publishes the pointer is one whose convention is the
// thing the exemption reads.
//
// WHAT REMAINS, stated rather than implied: the scan still reads the judged
// package's imports, so a package naming the type and importing none of its
// library's siblings is reported where a package importing one is not. That
// residue is smaller than the hole it replaces and it errs toward reporting,
// which is the direction an author can act on. Closing it needs the type's own
// module enumerated, which a pass cannot do and the framework can.
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
	return apiUsesPointer(pkg, named) || librarySiblingUsesPointer(pass, pkg, named)
}

// librarySiblingUsesPointer reports whether an imported package the type's own
// library owns hands the pointer out. The space a library owns is the parent of
// its package's import path — `formatter` and `parser` for a type declared in
// `github.com/vektah/gqlparser/v2/ast`. A single-segment path (`bytes`, `lib`)
// has `.` for a parent, which holds no import path at all, so it needs no case
// of its own: it establishes nothing for anybody and matches nothing.
//
// THE ANALYZED MODULE IS NEVER A CONVENTION SOURCE, and leaving that out was a
// hole the size of the one this scan closes. `path.Dir` of a type declared in a
// library's ROOT package is the owner namespace — `github.com/acme` for
// `github.com/acme/lib` — which contains `github.com/acme/app`, so an author
// working in that owner's namespace could write two lines in their own module,
// blank-import them, and silence the library's type. That is the same one-line
// evasion the whole scan is scoped to prevent, and the scope test alone does not
// catch it: only a package OUTSIDE the analyzed module can say what a library's
// convention is.
//
// What the path space still cannot separate is a library whose type sits at its
// module root from the owner namespace above it, so a DIFFERENT module under the
// same owner can establish a convention for it. That costs a published module a
// reviewer can see rather than a file in the tree being judged, which is the
// same line foreignConvention draws for its other blind spot.
func librarySiblingUsesPointer(pass *analysis.Pass, pkg *types.Package, named *types.Named) bool {
	space := importSpace(path.Dir(pkg.Path()))
	for _, imported := range pass.Pkg.Imports() {
		if localToModule(pass, imported) {
			continue
		}
		if space.holds(importPath(imported.Path())) && apiUsesPointer(imported, named) {
			return true
		}
	}
	return false
}

// importSpace is the import-path prefix a library owns.
type importSpace string

// importPath is one package's import path.
type importPath string

// holds reports whether an import path is the space itself or sits under it.
// The separator is required, so `github.com/a/bee` is outside the space
// `github.com/a/b`.
func (s importSpace) holds(candidate importPath) bool {
	return string(candidate) == string(s) || strings.HasPrefix(string(candidate), string(s)+"/")
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
