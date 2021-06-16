package bag

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	unitRE *regexp.Regexp
	// ErrConversion signals an error in unit conversion
	ErrConversion = errors.New("Conversion error")
)

func init() {
	unitRE = regexp.MustCompile("(.*?)(mm|cm|in|pt|px|pc|m)")
}

// Factor is the multiplyer to get ScaledPoints from  points to get the ScaledPoint unit
const Factor ScaledPoint = 0xffff

// A ScaledPoint is a 65535th of a DTP point
type ScaledPoint int

func (s ScaledPoint) String() string {
	return fmt.Sprintf("%.5g", float64(s)/float64(0xffff))
}

// Sp return the unit converted to ScaledPoint
func Sp(unit string) (ScaledPoint, error) {
	unit = strings.ToLower(unit)
	m := unitRE.FindAllStringSubmatch(unit, -1)
	if len(m) != 1 {
		return 0, fmt.Errorf("%w len(m) %d", ErrConversion, len(m))
	}
	if len(m[0]) != 3 {
		return 0, fmt.Errorf("%w len(m[0]) %d", ErrConversion, len(m[0]))
	}

	l, err := strconv.ParseFloat(m[0][1], 64)
	if err != nil {
		return 0, fmt.Errorf("%w parse float %s", ErrConversion, m[0][1])
	}
	unitstring := m[0][2]

	switch unitstring {
	case "pt":
		return ScaledPoint(l * 0xffff), nil
	case "in":
		return ScaledPoint(l * 72 * 0xffff), nil
	case "mm":
		// l = l / 10 [cm], l = l / 2.54 [in], l = l * 72 [pt]
		return ScaledPoint(l / 10 / 2.54 * 72 * 0xffff), nil
	case "cm":
		return ScaledPoint(l / 2.54 * 72 * 0xffff), nil
	case "m":
		return ScaledPoint(l * 100 / 2.54 * 72 * 0xffff), nil
	case "px":
		// 1/96th of an inch
		return ScaledPoint(l * 97 / 72 * 0xffff), nil
	case "pc":
		// pica, 12pt
		return ScaledPoint(l * 12 * 0xffff), nil
	default:
		return 0, ErrConversion

	}
}

// MustSp converts the unit to ScaledPoints. In case of an error, the function panics
func MustSp(unit string) ScaledPoint {
	val, err := Sp(unit)
	if err != nil {
		if errors.Is(err, ErrConversion) {
			fmt.Println(errors.Unwrap(err))
		}
		panic(err)
	}
	return val
}
