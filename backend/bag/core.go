package bag

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

var (
	unitRE = regexp.MustCompile("(.*?)(sp|mm|cm|in|pt|px|pc|m)")
	// ErrConversion signals an error in unit conversion
	ErrConversion = errors.New("Conversion error")
	// Logger is a zap logger which can be overridden from other packages
	Logger *zap.SugaredLogger
)

func init() {
	logger, _ := zap.NewProduction()
	Logger = logger.Sugar()
}

// A ScaledPoint is a 65535th of a DTP point
type ScaledPoint int

// Factor is the multiplier to get DTP points from scaled points.
const Factor ScaledPoint = 0xffff

// MaxSP is the largest dimension. Approx. 50,000,000 km on 64bit architecture.
// On 32bit architecture still about 11.5 meters.
const MaxSP = ScaledPoint(math.MaxInt)

// ScaledPointFromFloat converts the DTP point f to a ScaledPoint
func ScaledPointFromFloat(f float64) ScaledPoint {
	return ScaledPoint(f * float64(Factor))
}

// String converts the scaled point into a string, like Sprintf("%.3f")
// but with trailing zeroes (and possibly ".") removed.
func (s ScaledPoint) String() string {
	const precisionFactor = 100.0
	rounded := math.Round(precisionFactor*float64(s)/float64(Factor)) / precisionFactor
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

// ToPT returns the unit as a float64 DTP point. 2 * 0xffff returns 2.0
func (s ScaledPoint) ToPT() float64 {
	return float64(s) / float64(Factor)
}

// ToUnit returns the scaled points converted to the given unit. It raises an
// ErrConversion in case it cannot convert to the given unit.
func (s ScaledPoint) ToUnit(unit string) (float64, error) {
	const precisionFactor = 100000.0
	round := func(f float64) float64 {
		rounded := math.Round(precisionFactor*float64(s)*float64(f)/float64(Factor)) / precisionFactor
		return rounded
	}

	unit = strings.ToLower(unit)
	switch unit {
	case "sp":
		return float64(s), nil
	case "pt":
		return round(1.0), nil
	case "in":
		return round(1.0 / 72), nil
	case "mm":
		return round(1.0 * 10 * 2.54 / 72), nil
	case "cm":
		return round(1.0 * 2.54 / 72), nil
	case "m":
		return round(1.0 / 100 * 2.54 / 72), nil
	case "px":
		return round(1.0 / 96 * 72), nil
	case "pc":
		return round(1.0 / 12), nil
	default:
		return 0, ErrConversion
	}
}

// Sp return the unit converted to ScaledPoint. Unit can be a string like "1cm"
// or "12.5in". The units which are interpreted are pt, in, mm, cm, m, px and
// pc. A (wrapped) ErrConversion is returned in case of an error.
func Sp(unit string) (ScaledPoint, error) {
	if unit == "0" {
		return ScaledPoint(0), nil
	}
	unit = strings.ToLower(unit)
	m := unitRE.FindAllStringSubmatch(unit, -1)
	if len(m) != 1 {
		return 0, ErrConversion
	}
	if len(m[0]) != 3 {
		return 0, ErrConversion
	}

	l, err := strconv.ParseFloat(m[0][1], 64)
	if err != nil {
		return 0, fmt.Errorf("%w parse float %s", ErrConversion, m[0][1])
	}
	unitstring := m[0][2]

	switch unitstring {
	case "sp":
		return ScaledPoint(l), nil
	case "pt":
		return ScaledPoint(l * float64(Factor)), nil
	case "in":
		return ScaledPoint(l * 72 * float64(Factor)), nil
	case "mm":
		// l = l / 10 [cm], l = l / 2.54 [in], l = l * 72 [pt]
		return ScaledPoint(l / 10 / 2.54 * 72 * float64(Factor)), nil
	case "cm":
		return ScaledPoint(l / 2.54 * 72 * float64(Factor)), nil
	case "m":
		return ScaledPoint(l * 100 / 2.54 * 72 * float64(Factor)), nil
	case "px":
		// 1/96th of an inch
		return ScaledPoint(l * 96 / 72 * float64(Factor)), nil
	case "pc":
		// pica, 12pt
		return ScaledPoint(l * 12 * float64(Factor)), nil
	default:
		return 0, ErrConversion
	}
}

// MustSp converts the unit to ScaledPoints. In case of an error, the function
// panics.
func MustSp(unit string) ScaledPoint {
	val, err := Sp(unit)
	if err != nil {
		if errors.Is(err, ErrConversion) {
			Logger.Error(err.Error())
		}
		panic(err)
	}
	return val
}

// Max returns the maximum of the two scaled points.
func Max(a, b ScaledPoint) ScaledPoint {
	if a > b {
		return a
	}
	return b
}

// Min returns the minimum of the two scaled points.
func Min(a, b ScaledPoint) ScaledPoint {
	if a < b {
		return a
	}
	return b
}

// MultiplyFloat returns a multiplied by f.
func MultiplyFloat(a ScaledPoint, f float64) ScaledPoint {
	return ScaledPoint(a.ToPT() * f * float64(Factor))
}
