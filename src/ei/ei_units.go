package ei

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type unit struct {
	Symbol string
	Oom    int
}

var units = []unit{
	{"K", 3}, {"M", 6}, {"B", 9}, {"T", 12}, {"q", 15}, {"Q", 18}, {"s", 21}, {"S", 24},
	{"o", 27}, {"N", 30}, {"d", 33}, {"U", 36}, {"D", 39}, {"Td", 42}, {"qd", 45},
	{"Qd", 48}, {"sd", 51}, {"Sd", 54}, {"Od", 57}, {"Nd", 60}, {"V", 63}, {"uV", 66},
	{"dV", 69}, {"tV", 72}, {"qV", 75}, {"QV", 78}, {"sV", 81}, {"SV", 84}, {"OV", 87},
	{"NV", 90}, {"tT", 93},
}

var oom2symbol = make(map[int]string)
var symbol2oom = make(map[string]int)
var minOom, maxOom int

func init() {
	for _, u := range units {
		oom2symbol[u.Oom] = u.Symbol
		symbol2oom[u.Symbol] = u.Oom
	}
	minOom = units[0].Oom
	maxOom = units[len(units)-1].Oom
}

var valueWithUnitRegExpPattern = fmt.Sprintf(`\b(?P<value>\d+(\.\d+)?)\s*(?P<unit>%s)\b`,
	strings.Join(func() []string {
		symbols := make([]string, len(units))
		for i, u := range units {
			symbols[i] = u.Symbol
		}
		return symbols
	}(), "|"),
)

// var valueWithUnitRegExp = regexp.MustCompile(valueWithUnitRegExpPattern)
var valueWithOptionalUnitRegExpPattern = fmt.Sprintf(`\b(?P<value>\d+(\.\d+)?)\s*(?P<unit>%s)?\b`,
	strings.Join(func() []string {
		symbols := make([]string, len(units))
		for i, u := range units {
			symbols[i] = u.Symbol
		}
		return symbols
	}(), "|"),
)

//var valueWithOptionalUnitRegExp = regexp.MustCompile(valueWithOptionalUnitRegExpPattern)

// ParseValueWithUnit parses a string containing a number and an optional unit.
func ParseValueWithUnit(s string, unitRequired bool) (float64, error) {
	var re *regexp.Regexp
	if unitRequired {
		re = regexp.MustCompile("^" + valueWithUnitRegExpPattern + "$")
	} else {
		re = regexp.MustCompile("^" + valueWithOptionalUnitRegExpPattern + "$")
	}

	match := re.FindStringSubmatch(s)
	if match == nil {
		return 0, fmt.Errorf("no match found")
	}

	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}

	unit := match[3]
	if unit == "" {
		return value, nil
	}

	oom, ok := symbol2oom[unit]
	if !ok {
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return value * math.Pow10(oom), nil
}

// FormatEIValue formats a number in scientific notation with the given options.
func FormatEIValue(x float64, options map[string]any) string {
	trim := options["trim"] == true
	decimals := 3
	if d, ok := options["decimals"].(int); ok {
		decimals = d
	}
	scientific := options["scientific"] == true

	if math.IsNaN(x) {
		return "NaN"
	}
	if x < 0 {
		return "-" + FormatEIValue(-x, options)
	}
	if math.IsInf(x, 0) {
		return "infinity"
	}

	oom := math.Log10(x)
	if oom < float64(minOom) {
		return strconv.FormatFloat(x, 'f', 0, 64)
	}

	oomFloor := math.Floor(oom)
	if oom+1e-9 >= oomFloor+1 {
		oomFloor++
	}
	oomFloor -= math.Mod(oomFloor, 3)
	if int(oomFloor) > maxOom {
		oomFloor = float64(maxOom)
	}

	principal := x / math.Pow10(int(oomFloor))
	var numpart string
	if principal < 1e21 {
		if precision, ok := options["precision"].(int); ok {
			numpart = strconv.FormatFloat(principal, 'g', precision, 64)
		} else {
			numpart = strconv.FormatFloat(principal, 'f', decimals, 64)
		}
	} else {
		numpart = strconv.FormatFloat(principal, 'g', 4, 64)
	}

	if trim {
		numpart = strings.TrimRight(strings.TrimRight(numpart, "0"), ".")
	}

	if scientific {
		return fmt.Sprintf("%s×10^%d", numpart, int(oomFloor))
	}
	return numpart + oom2symbol[int(oomFloor)]
}

// FmtApprox formats a number in scientific notation with 3 decimal places.
func FmtApprox(n float64) string {
	if n == 0 {
		return "0"
	}
	return FormatEIValue(n, map[string]interface{}{"decimals": 3})
}
