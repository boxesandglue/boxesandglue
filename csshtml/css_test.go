package csshtml

import (
	"testing"
)

func TestNestedAtrule(t *testing.T) {

	str := `
	@page {
		size: a5;
		@bottom-right-corner {
			border: 4pt solid green;
			border-bottom-color: rebeccapurple;
		}

		/* @top-left-corner {
			border: 1pt solid green;
			border-bottom-color: rebeccapurple;
		} */

	@top-right-corner {
			border: 3pt solid green;
			border-bottom-color: rebeccapurple;
		}

		@bottom-left-corner {
			border: 2pt solid green;
			border-bottom-color: rebeccapurple;
		}

	}`
	toks := ParseCSSString(str)
	bl := ConsumeBlock(toks, false)
	if len(bl.ChildAtRules[0].ChildAtRules) != 3 {
		t.Errorf("want 3 child @ rules, got %d", len(bl.ChildAtRules[0].ChildAtRules))
	}
}
