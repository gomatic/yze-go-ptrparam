// Derived-type resolution: a type DEFINED over another named type has that
// type's layout, and so its copy hazards, whatever methods it drops.
package ptrparam

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

// decision is what a verdict needs besides the parameter itself.
type decision struct {
	pass  *analysis.Pass
	allow map[string]bool
	over  definitions
}

// definitions maps each type the analyzed package DEFINES over another named
// type to that named type: `type MyBuilder strings.Builder` maps MyBuilder to
// strings.Builder. An alias is not in it, because an alias is not a new type.
type definitions map[types.Type]*types.Named

// definedOver collects the analyzed package's definitions over named types.
func definedOver(pass *analysis.Pass, insp *inspector.Inspector) definitions {
	over := definitions{}
	insp.Preorder([]ast.Node{(*ast.TypeSpec)(nil)}, func(n ast.Node) {
		spec := n.(*ast.TypeSpec)
		source, ok := types.Unalias(pass.TypesInfo.TypeOf(spec.Type)).(*types.Named)
		if !ok || spec.Assign.IsValid() {
			return
		}
		over[pass.TypesInfo.TypeOf(spec.Name)] = source
	})
	return over
}

// conventioned reports whether a pointer is the calling convention of the
// parameter's own type, or of a type it is defined over.
func conventioned(d decision, t *types.Named) bool {
	return conventionedHere(d, t) || inheritsHazard(d.over, d.over[t.Origin()])
}

// conventionedHere applies the three exemptions that answer for a type on its
// own account: the -allow list, the pointer-idiomatic criteria, and a foreign
// library's published convention. A type from the universe scope (`error`) has
// no package to qualify its name with and takes none of them.
func conventionedHere(d decision, t *types.Named) bool {
	if t.Obj().Pkg() == nil {
		return false
	}
	name := t.Obj().Pkg().Path() + "." + t.Obj().Name()
	return d.allow[name] || pointerIdiomatic(t) || foreignConvention(d.pass, t)
}

// inheritsHazard walks the chain of types the parameter's type is DEFINED
// over and reports whether any link carries a copy hazard the definition
// inherits along with the layout.
//
// WHAT IS INHERITED IS THE LAYOUT, SO WHAT IS INHERITED IS THE HAZARD, and
// nothing else. `type MyBuilder strings.Builder` copies strings.Builder byte
// for byte and inherits none of the methods that ANNOUNCE the hazard, so
// judging the definition alone reported it — and the remedy prescribed there
// is not merely expensive, it is wrong: taking it was built and RUN, and it
// panics with `strings: illegal use of non-zero Builder copied by value`,
// while go vet exits 0 on the remedied source. `type MyBuf bytes.Buffer` and
// `type MyRand rand.Rand` are the same shape and corrupt silently instead.
// That is pointerIdiomatic's question — a lock in the layout, or a method set
// that exists only on the pointer — and it is the only one asked of a link.
//
// THE OTHER TWO EXEMPTIONS DO NOT TRANSFER, and applying them here was a
// silent widening of both. A foreign library's convention is the claim that
// forcing values onto ITS type would be wrong code, because its own API hands
// the pointer out; no API anywhere mentions a type declared in the analyzed
// module, so nothing forces a pointer on it and passing it by value is code
// the library never sees. An -allow entry names one `pkgpath.Name`, and
// inheriting it silences types it does not name — a configured disablement
// spreading past what was configured. Both left one `type` line buying silence
// for a type carrying no hazard at all, which is the cheapest evasion in this
// analyzer.
//
// The walk needs no cycle guard and carries none: Go rejects a definition
// cycle outright — `type A B; type B A` is "invalid recursive type", verified
// rather than assumed — so following definitions terminates. That claim covers
// this walk alone; the type walk in semantic.go descends through constraints
// as well, where Go rejects nothing, and carries its own guard.
//
// IT STOPS AT THE PACKAGE BOUNDARY, and that is a hole rather than a design.
// definedOver reads the analyzed package's own TypeSpecs, because a pass has
// the syntax of no other package, so the identical `type MyBuilder
// strings.Builder` declared one package away is still reported and its
// prescribed remedy still panics. The chain is keyed by the generic ORIGIN,
// which is what a TypeSpec names, and looked up by it, so an instantiation of
// a generic definition resolves like any other.
func inheritsHazard(over definitions, source *types.Named) bool {
	for ; source != nil; source = over[source.Origin()] {
		if pointerIdiomatic(source) {
			return true
		}
	}
	return false
}
