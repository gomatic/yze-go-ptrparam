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

// conventioned reports whether a pointer is the calling convention of t or of
// any type t is defined over.
//
// `type MyBuilder strings.Builder` copies strings.Builder byte for byte, so it
// carries the copy hazard that makes a pointer strings.Builder's convention —
// and it inherits none of the methods that ANNOUNCE that convention, so
// judging the definition alone reported it. The remedy prescribed there is not
// merely expensive, it is wrong: taking it was built and RUN, and it panics
// with `strings: illegal use of non-zero Builder copied by value`, while go
// vet exits 0 on the remedied source. `type MyBuf bytes.Buffer` and
// `type MyRand rand.Rand` are the same shape and corrupt silently instead.
//
// The walk needs no cycle guard and carries none: Go rejects a definition
// cycle outright — `type A B; type B A` is "invalid recursive type", verified
// rather than assumed — so following definitions terminates.
func conventioned(d decision, t *types.Named) bool {
	for t != nil {
		if t.Obj().Pkg() != nil && conventionedHere(d, t) {
			return true
		}
		t = d.over[t]
	}
	return false
}

// conventionedHere applies the three exemptions to one type.
func conventionedHere(d decision, t *types.Named) bool {
	name := t.Obj().Pkg().Path() + "." + t.Obj().Name()
	return d.allow[name] || pointerIdiomatic(t) || foreignConvention(d.pass, t)
}
