package theme

import (
	"github.com/vivaui/viva/ui"
	"testing"
)

// Expected RGB triples were captured from java.awt.Color using the exact
// DefaultColorGenerator formula, including signed 32-bit overflow.
func TestGraphColorMatchesJava(t *testing.T) {
	for _, tc := range []struct {
		id      int
		r, g, b uint8
	}{
		{1, 99, 155, 166}, {2, 118, 99, 166}, {-1, 156, 166, 99}, {17, 99, 166, 99},
		{-1081267614, 129, 166, 99}, {3343801, 99, 155, 166}, {2147483647, 156, 166, 99}, {-2147483648, 139, 166, 99},
	} {
		for _, dark := range []bool{false, true} {
			if got, want := GraphColor(tc.id, dark), ui.RGB(tc.r, tc.g, tc.b); got != want {
				t.Errorf("id %d dark %v = %v, want %v", tc.id, dark, got, want)
			}
		}
	}
	if GraphColor(0, false) != ui.RGB(0, 0, 0) || GraphColor(0, true) != ui.RGB(255, 255, 255) {
		t.Fatal("default must follow JBColor.BLACK")
	}
}
