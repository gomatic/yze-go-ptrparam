package ptrparam

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSplitNonEmptyProducesNoMatchableEmptyEntry names the claim
// splitNonEmpty's empty check rests on, because that claim is that no corpus
// case can discriminate it, and a claim like that has to be pinned somewhere
// rather than asserted in a comment. Without the check an unset flag yields
// one entry, "", and the allow-list gains a key that no lookup — a package
// path, a dot and a type name — can equal. It keeps the map honest and
// decides no verdict, which is why it takes this test and not a fixture.
func TestSplitNonEmptyProducesNoMatchableEmptyEntry(t *testing.T) {
	t.Parallel()
	assert.Nil(t, splitNonEmpty(""), "an unset flag is no entries, not one empty entry")
	assert.Equal(t, []string{"a.B", "c.D"}, splitNonEmpty("a.B,c.D"))
	assert.Empty(t, buildAllow(""), "an unset flag allows nothing")
	assert.Equal(t, map[string]bool{"a.B": true}, buildAllow("a.B"))
}
