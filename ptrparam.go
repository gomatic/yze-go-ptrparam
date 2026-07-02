// Package ptrparam provides a go/analysis analyzer enforcing the gomatic Go
// immutability standard: function parameters are passed by value, never by
// pointer, unless a pointer is the pointed-to type's idiomatic calling
// convention — a standard-library type conventionally passed by pointer, the
// sanctioned CLI framework's *cli.Command (urfave/cli/v3 imposes it in every
// Action/Before/After signature), or a type parameter (a generic seam whose
// instantiations the analyzer cannot judge).
package ptrparam

import (
	"go/ast"
	"go/types"
	"strings"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// allowedPointerParams are the types conventionally passed by pointer: the
// standard-library types where a pointer is the idiomatic calling convention,
// plus the sanctioned CLI framework's command type, whose pointer-taking
// callback signatures (Action/Before/After/ExitErrHandler) urfave/cli/v3
// itself imposes on every conforming CLI.
var allowedPointerParams = map[string]bool{
	"log/slog.Logger":                  true,
	"testing.T":                        true,
	"testing.B":                        true,
	"testing.F":                        true,
	"testing.M":                        true,
	"sync.WaitGroup":                   true,
	"os.File":                          true,
	"os.Root":                          true,
	"net/http.Request":                 true,
	"net/http.Response":                true,
	"bytes.Buffer":                     true,
	"strings.Builder":                  true,
	"text/template.Template":           true,
	"html/template.Template":           true,
	"crypto/tls.Config":                true,
	"database/sql.DB":                  true,
	"database/sql.Tx":                  true,
	"database/sql.Stmt":                true,
	"github.com/urfave/cli/v3.Command": true,
}

// allowExtra is the configurable allow-list of additional fully-qualified
// pointer-parameter types (pkgpath.Name), set via the -allow flag or config.
var allowExtra string

// Analyzer reports pointer parameters that are not idiomatic standard-library types.
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
	URL:        "https://docs.gomatic.dev/yze/go/ptrparam",
	Analyzer:   Analyzer,
}

// run reports each disallowed pointer parameter.
func run(pass *analysis.Pass) (any, error) {
	allow := buildAllow(allowExtra)
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{(*ast.FuncType)(nil)}, func(n ast.Node) {
		for _, field := range n.(*ast.FuncType).Params.List {
			check(pass, allow, field)
		}
	})
	return nil, nil
}

// buildAllow merges the baked-in idiomatic pointer types with the configured extras.
func buildAllow(extra string) map[string]bool {
	allow := make(map[string]bool, len(allowedPointerParams))
	for name := range allowedPointerParams {
		allow[name] = true
	}
	for _, name := range splitNonEmpty(extra) {
		allow[name] = true
	}
	return allow
}

func splitNonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// check reports a parameter whose type is a non-idiomatic pointer.
func check(pass *analysis.Pass, allow map[string]bool, field *ast.Field) {
	star, ok := paramType(field).(*ast.StarExpr)
	if !ok || allowedPointer(allow, pass, star.X) {
		return
	}
	pass.Reportf(
		star.Pos(),
		"pointer parameter; pass by value unless a pointer is the type's idiomatic calling convention",
	)
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

// allowedPointer reports whether the pointed-to type expression names an
// allow-listed type or a type parameter. A pointer to a type parameter is a
// generic seam — the function cannot know its instantiations, and the pointer
// is how a generic function binds to a caller-owned value (e.g. a flag
// destination) — so it is never reported.
func allowedPointer(allow map[string]bool, pass *analysis.Pass, x ast.Expr) bool {
	switch t := types.Unalias(pass.TypesInfo.TypeOf(x)).(type) {
	case *types.TypeParam:
		return true
	case *types.Named:
		return t.Obj().Pkg() != nil && allow[t.Obj().Pkg().Path()+"."+t.Obj().Name()]
	default:
		return false
	}
}
