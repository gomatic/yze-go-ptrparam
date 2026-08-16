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
// There are six exemptions and no others. Five are decided from the type; the
// sixth is decided from what the loader happened to materialise and is stated
// at the end, where it used to be miscounted as a limitation.
//
//   - Pointer-idiomatic, decided from the type itself and applying to any
//     package including the analyzed module's own (semantic.go): the type is
//     uncopyable under go vet's copylocks criterion — a struct whose POINTER
//     is a sync.Locker while its value is not, directly or through a struct
//     field or array element — or every exported method it declares takes the
//     pointer receiver, so a value carries no usable API. The second is
//     forgeable by design and its forgery is charged by yze/ptrrecv WHEN THE
//     TYPE DECLARING THE METHOD IS THE PARAMETER'S OWN — the author who writes
//     the pointer receiver is the one the finding lands on. It is NOT charged
//     when the criterion is reached through the definition chain below: there
//     the receiver was written on the source type, whose author may have had
//     their own reason for it and was already paying that finding, so the
//     definition buys silence at nobody's expense. That is
//     ptrparam.inherited-hazard-is-a-layout-property (k1n8261k), open, and it
//     is stated here rather than only in derived.go because docs/s03.md sends
//     an enumerator to this comment. The first criterion costs the marker go
//     vet then reports every copy of the type against.
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
//     whose OWN PACKAGE's exported API hands out or accepts a pointer to it —
//     in a function or method signature, an interface method, an exported
//     struct field, or a callback field, directly or one container level deep.
//     The analyzed module's own types are its own design responsibility and
//     never gain it, and neither does a named type over a basic underlying,
//     where `*T` is an out-parameter (flag.DurationVar's *time.Duration)
//     rather than a passing convention.
//
//     NO OTHER PACKAGE IS READ — not a sibling, not one inside the library's
//     import namespace, not one the judged file imports. Both narrower
//     readings were tried and both were forgeable from the tree being judged:
//     an unrestricted scan of the imports made `_ "weaver"` a disablement, and
//     restricting it to the library's own namespace fell to a go.mod `replace`,
//     which makes an import path a purely local claim (k1n828c6). A pass
//     carries no module identity for an imported package, so "a sibling, but a
//     published one" is a distinction this analyzer has no instrument for.
//     The cost is stated because it is real: a library that publishes the
//     pointer only from a package beside the type — gqlparser's
//     `*ast.QueryDocument`, whose own `ast` package mentions it nowhere — is
//     REPORTED here, and the value the diagnostic prescribes silently loses
//     mutations made through the alias. The move left to that author is an
//     -allow entry, which an inventory can read, rather than an import line,
//     which leaves no entry anywhere.
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
// A SIXTH EXEMPTION, named as one because that is what it is. Where go/types
// did not materialise a foreign type's own package — it reached the type
// through another package's alias re-export and loaded nothing else — the
// analyzer exempts the parameter. This comment called that "a scope
// limitation, which is not an exemption" and that was wrong twice over: it is
// the `return true` branch of foreignConvention, it produces silence at zero
// cost, and it disagreed with foreign.go's own comment, which had already been
// corrected to call it a disablement channel. An enumerator following
// docs/s03.md reads THIS comment, so counting it out of the list is how a
// shape passes every instrument by not being looked for.
//
// It is import-list-dependent and it is DRIVER-DEPENDENT, which is the part
// worth writing down: a four-line alias package in the author's own module,
// `type Doc = ast.Doc`, silences the library's type under `go vet -vettool`
// while the same source reports under a packages.Load driver, and one blank
// import of the library flips the vet verdict back. Neither polarity is
// import-independent — blindness cannot tell "this library publishes no
// pointer convention" from "this library was not loaded" — so this is a choice
// between two channels rather than the absence of one. The repair is a loader
// that materialises the type's own package, which belongs to go-yze.
// ptrparam.verdict-does-not-follow-the-loaders-reach (k1n81qeg), open.
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
