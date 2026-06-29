package ptrparam_test

import (
	"testing"

	ptrparam "github.com/gomatic/yze-go-ptrparam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDisallowedPointerParametersAreReported(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ptrparam.Analyzer, "a")
}

func TestRegistrationIsWellFormed(t *testing.T) {
	assert.NoError(t, ptrparam.Registration.Validate())
	assert.Equal(t, "yze/go/ptrparam", ptrparam.Registration.RuleID())
	assert.Same(t, ptrparam.Analyzer, ptrparam.Registration.Analyzer)
}

func TestAllowFlagPermitsConfiguredPointerTypes(t *testing.T) {
	require.NoError(t, ptrparam.Analyzer.Flags.Set("allow", "b.Special"))
	t.Cleanup(func() { _ = ptrparam.Analyzer.Flags.Set("allow", "") })

	analysistest.Run(t, analysistest.TestData(), ptrparam.Analyzer, "b")
}
