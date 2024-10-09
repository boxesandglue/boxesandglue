package bag

import (
	"fmt"
	"testing"
)

func TestUnits(t *testing.T) {
	// units := []string{"mm", "cm", "in", "pt", "px", "pc", "m"}
	units := []string{"pt", "in", "mm", "cm", "m", "px", "pc", "sp"}
	for _, val := range []int{1, 0, -1, 2000} {
		for _, unit := range units {
			a := MustSP(fmt.Sprintf("%d%s", val, unit))
			b, err := a.ToUnit(unit)
			if err != nil {
				t.Errorf(err.Error())
			}
			if float64(val) != b {
				t.Errorf("%v a.ToUnit(%s) = %f, want %d", a, unit, b, val)
			}

		}
	}
}
