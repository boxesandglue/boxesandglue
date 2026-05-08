package frontend

import "testing"

// TestResolveLogicalAlignment guards the two-defaults contract that splits
// the TeX-style API surface from the CSS-spec API surface:
//
//   - HAlignDefault (Go zero value, "no SettingHAlign set") must resolve to
//     HAlignJustified regardless of direction. This is what every
//     boxesandglue Go caller sees when it never touches SettingHAlign.
//     Regression history: between 2026-05-06 and 2026-05-08 this case was
//     accidentally folded into the CSS-logical "start" path, silently
//     turning every Go-API paragraph ragged-right.
//   - HAlignStart / HAlignEnd are CSS Text 3 §7 logical keywords and must
//     resolve direction-aware. htmlbag funnels here so unstyled HTML follows
//     CSS spec (Arabic right, Latin left) without explicit opt-in.
//   - Physical keywords must pass through unchanged — author-set values do
//     not flip on direction.
func TestResolveLogicalAlignment(t *testing.T) {
	cases := []struct {
		name string
		in   HorizontalAlignment
		dir  Direction
		want HorizontalAlignment
	}{
		{"default LTR → justified", HAlignDefault, DirectionLTR, HAlignJustified},
		{"default RTL → justified", HAlignDefault, DirectionRTL, HAlignJustified},

		{"start LTR → left", HAlignStart, DirectionLTR, HAlignLeft},
		{"start RTL → right", HAlignStart, DirectionRTL, HAlignRight},
		{"end LTR → right", HAlignEnd, DirectionLTR, HAlignRight},
		{"end RTL → left", HAlignEnd, DirectionRTL, HAlignLeft},

		{"left LTR stays left", HAlignLeft, DirectionLTR, HAlignLeft},
		{"left RTL stays left", HAlignLeft, DirectionRTL, HAlignLeft},
		{"right LTR stays right", HAlignRight, DirectionLTR, HAlignRight},
		{"right RTL stays right", HAlignRight, DirectionRTL, HAlignRight},
		{"center LTR stays center", HAlignCenter, DirectionLTR, HAlignCenter},
		{"center RTL stays center", HAlignCenter, DirectionRTL, HAlignCenter},
		{"justified LTR stays justified", HAlignJustified, DirectionLTR, HAlignJustified},
		{"justified RTL stays justified", HAlignJustified, DirectionRTL, HAlignJustified},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveLogicalAlignment(tc.in, tc.dir); got != tc.want {
				t.Errorf("resolveLogicalAlignment(%v, %v) = %v, want %v",
					tc.in, tc.dir, got, tc.want)
			}
		})
	}
}
