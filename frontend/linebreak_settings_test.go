package frontend

import (
	"io"
	"testing"
)

// A tolerance or hyphen penalty of 0 in the settings is a value, not "unset":
// Tolerance 0 accepts only lines that fit exactly, Hyphenpenalty 0 makes a
// hyphen cost nothing. Missing settings and a negative tolerance keep the
// defaults (htmlbag#104).
func TestLinebreakSettingsZero(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		settings  TypesettingSettings
		tolerance float64
		penalty   int
	}{
		{"unset", TypesettingSettings{}, 4, 50},
		{"zero", TypesettingSettings{SettingLinebreakTolerance: 0.0, SettingHyphenPenalty: 0}, 0, 0},
		{"set", TypesettingSettings{SettingLinebreakTolerance: 2.5, SettingHyphenPenalty: 200}, 2.5, 200},
		{"negative tolerance", TypesettingSettings{SettingLinebreakTolerance: -1.0}, 4, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			te := NewText()
			te.Settings = tc.settings
			ls := fe.linebreakSettings(te, &Options{})
			if ls.Tolerance != tc.tolerance || ls.Hyphenpenalty != tc.penalty {
				t.Errorf("Tolerance %v, Hyphenpenalty %d; want %v, %d", ls.Tolerance, ls.Hyphenpenalty, tc.tolerance, tc.penalty)
			}
		})
	}
}
