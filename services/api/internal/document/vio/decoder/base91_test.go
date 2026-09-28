package decoder

import "testing"

func TestBase91AlphabetHasExpectedLength(t *testing.T) {
	if len(base91Alphabet) != 91 {
		t.Fatalf("alphabet length = %d, want 91", len(base91Alphabet))
	}
}
