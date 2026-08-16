// Package ptrparam provides a go/analysis analyzer enforcing the gomatic Go
// immutability standard: function parameters are passed by value, never by
// pointer, unless a pointer is the pointed-to type's idiomatic calling
// convention.
//
// A parameter is judged on its TYPE, not on its spelling: `*T`, an alias of
// `*T`, a defined type whose underlying is `*T`, and an instantiated generic
// alias are one rule, because they are one type and no call site can tell
// them apart.
//
// There are five exemptions and no others.
//
//   - Pointer-idiomatic, decided from the type itself and applying to any
//     package including the analyzed module's own (semantic.go): the type is
//     uncopyable under go vet's copylocks criterion — a struct whose POINTER
//     is a sync.Locker while its value is not, directly or through a struct
//     field or array element — or every exported method it declares takes the
//     pointer receiver, so a value carries no usable API. The second is
//     forgeable by design and its forgery is charged by yze/ptrrecv; the first
//     costs the marker go vet then reports every copy of the type against.
//
//   - An inherited copy hazard (derived.go): a type DEFINED over another named
//     type — `type MyBuilder strings.Builder` — has that type's layout and so
//     its copy hazards, while inheriting none of the methods that announce
//     them. This one DOES apply to the analyzed module's own types, and it is
//     the only exemption that does so on the strength of another type: the
//     hazard is in the layout, and the layout is what a definition copies.
//     Only the pointer-idiomatic question above is asked of a link in that
//     chain, never the two below it — a foreign library's convention is about
//     the library's own type and no signature anywhere can be handed the local
//     one, and an -allow entry names a single `pkgpath.Name`. Inheriting
//     either made one `type` line the cheapest silence available here.
//
//   - Foreign convention (foreign.go): a type from OUTSIDE the analyzed module
//     whose own package's exported API hands out or accepts a pointer to it —
//     in a function or method signature, an interface method, an exported
//     struct field, or a callback field, directly or one container level deep.
//     The analyzed module's own types are its own design responsibility and
//     never gain it, and neither does a named type over a basic underlying,
//     where `*T` is an out-parameter (flag.DurationVar's *time.Duration)
//     rather than a passing convention.
//
//   - A type parameter: a generic seam whose instantiations the analyzer
//     cannot judge, and the pointer is how a generic function binds to a
//     caller-owned value.
//
//   - The -allow flag (`analyzers: {ptrparam: {allow: [...]}}` under stickler),
//     a comma-separated list of fully-qualified `pkgpath.Name` types. This is
//     a SILENT disablement channel: a configured entry produces no output, no
//     count and no ratchet, and a misspelt entry is accepted without complaint
//     and is simply dead. It is named here because an exemption nobody can
//     enumerate is one nobody reviews.
//
// One scope limitation, which is not an exemption: where go/types did not
// materialise a foreign type's own package — it reached the type through
// another package's alias re-export and loaded nothing else — the analyzer
// renders no verdict on it, because the alternative is a verdict that follows
// the analyzed file's import list rather than its code.
package ptrparam

import (
	"go/ast"
	"go/types"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer reports pointer parameters whose pointed-to type has no pointer
// calling convention.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name: "ptrparam",
		Doc: "reports pointer parameters unless a pointer is the pointed-to " +
			"type's idiomatic calling convention",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      run,
	}
	a.Flags.StringVar(
		&allowExtra,
		"allow",
		"",
		"comma-separated extra fully-qualified pointer-parameter types (pkgpath.Name)",
	)
	return a
}

// Registration declares this analyzer to the yze framework.
var Registration = goyze.Registration{
	Name:       "ptrparam",
	Categories: []goyze.Category{"immutability"},
	URL:        "https://docs.gomatic.dev/yze/ptrparam",
	Analyzer:   Analyzer,
}

// run reports each disallowed pointer parameter.
func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	d := decision{pass: pass, allow: buildAllow(allowCSV(allowExtra)), over: definedOver(pass, insp)}
	insp.Preorder([]ast.Node{(*ast.FuncType)(nil)}, func(n ast.Node) {
		for _, field := range n.(*ast.FuncType).Params.List {
			check(d, field)
		}
	})
	return nil, nil
}

// check reports a parameter whose type is a non-idiomatic pointer.
func check(d decision, field *ast.Field) {
	expr := paramType(field)
	ptr, ok := pointerType(d.pass.TypesInfo.TypeOf(expr))
	if !ok || allowedPointer(d, ptr.Elem()) {
		return
	}
	d.pass.Reportf(
		expr.Pos(),
		"pointer parameter; pass by value unless a pointer is the type's idiomatic calling convention",
	)
}

// pointerType unwraps a type expression's TYPE to the pointer it denotes,
// seeing through an alias (identical to the pointer in every way Go
// recognises) and through a defined type whose underlying is a pointer. The
// rule is about what a parameter IS, not how it is spelled: matching the
// `*T` syntax alone lets one `type` line take a parameter out of the rule
// without changing a single call site.
func pointerType(t types.Type) (*types.Pointer, bool) {
	switch u := types.Unalias(t).(type) {
	case *types.Pointer:
		return u, true
	case *types.Named:
		ptr, ok := u.Underlying().(*types.Pointer)
		return ptr, ok
	default:
		return nil, false
	}
}

// paramType returns the type expression to inspect for a parameter field,
// unwrapping a variadic parameter's ellipsis to its element type so that
// `...*T` is treated as a pointer parameter.
func paramType(field *ast.Field) ast.Expr {
	if ellipsis, ok := field.Type.(*ast.Ellipsis); ok {
		return ellipsis.Elt
	}
	return field.Type
}

// allowedPointer reports whether the pointed-to type is an allow-listed type,
// a type parameter, or a semantically pointer-idiomatic type. A pointer to a
// type parameter is a generic seam — the function cannot know its
// instantiations, and the pointer is how a generic function binds to a
// caller-owned value (e.g. a flag destination) — so it is never reported.
func allowedPointer(d decision, elem types.Type) bool {
	switch t := types.Unalias(elem).(type) {
	case *types.TypeParam:
		return true
	case *types.Named:
		return conventioned(d, t)
	default:
		return false
	}
}
