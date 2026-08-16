package ptrparam_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	ptrparam "github.com/gomatic/yze-go-ptrparam"
)

func TestDisallowedPointerParametersAreReported(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ptrparam.Analyzer,
		"a", "c", "constraintcycle", "d", "entrypoint", "forged", "neighbour", "nosibling", "sibling")
}

func TestRegistrationIsWellFormed(t *testing.T) {
	assert.NoError(t, ptrparam.Registration.Validate())
	assert.Equal(t, "yze/ptrparam", ptrparam.Registration.RuleID())
	assert.Same(t, ptrparam.Analyzer, ptrparam.Registration.Analyzer)
}

func TestAllowFlagPermitsConfiguredPointerTypes(t *testing.T) {
	require.NoError(t, ptrparam.Analyzer.Flags.Set("allow", "b.Special"))
	t.Cleanup(func() { _ = ptrparam.Analyzer.Flags.Set("allow", "") })

	analysistest.Run(t, analysistest.TestData(), ptrparam.Analyzer, "b")
}

// TestDiagnosticNamesThePointerAndSaysWhy owns the two things the corpus
// structurally cannot read. corpus.go's Finding is {rule, path, line} — no
// message and no column — and analysistest matches a `want` by line, so
// moving the report from the pointer to the parameter's NAME, or rewording
// the message into something an author cannot act on, is invisible to every
// case in testdata. Both are part of the standard: the diagnostic has to
// point at the pointer that is the finding, and it has to name the remedy.
func TestDiagnosticNamesThePointerAndSaysWhy(t *testing.T) {
	t.Parallel()
	// `func takes(param *Plain)`: counted off the literal rather than guessed,
	// the parameter NAME starts at column 12 and the '*' at column 18. A report
	// at field.Pos() and one at the pointer land on the same LINE and different
	// columns, which is why only a column assertion can tell them apart.
	got := diagnose(t, `package p

type Plain struct{ n int }

func takes(param *Plain) { _ = param }
`)
	require.Len(t, got, 1)
	assert.Equal(t,
		"pointer parameter; pass by value unless a pointer is the type's idiomatic calling convention",
		got[0].Message)
	assert.Equal(t, 5, got[0].Line, "the parameter's line")
	assert.Equal(t, 18, got[0].Column, "the '*', not the parameter name at column 12")
}

// reported is one diagnostic, resolved to a position.
type reported struct {
	Message string
	Line    int
	Column  int
}

// diagnose type-checks src and runs the analyzer over it directly, which is
// the only way to see a diagnostic's message and column: analysistest asserts
// neither.
func diagnose(t *testing.T, src string) []reported {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	require.NoError(t, err)

	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("example.test/p", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	var got []reported
	pass := &analysis.Pass{
		Analyzer:  ptrparam.Analyzer,
		Fset:      fset,
		Files:     []*ast.File{file},
		Pkg:       pkg,
		TypesInfo: info,
		ResultOf:  map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New([]*ast.File{file})},
		Report: func(d analysis.Diagnostic) {
			at := fset.Position(d.Pos)
			got = append(got, reported{Message: d.Message, Line: at.Line, Column: at.Column})
		},
	}
	_, err = ptrparam.Analyzer.Run(pass)
	require.NoError(t, err)
	return got
}
