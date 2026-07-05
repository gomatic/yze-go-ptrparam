package ptrparam_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	ptrparam "github.com/gomatic/yze-go-ptrparam"
)

func TestDisallowedPointerParametersAreReported(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ptrparam.Analyzer, "a", "c", "d")
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
