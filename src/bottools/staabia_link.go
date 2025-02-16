package bottools

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// Taken from https://srsandbox-staabmia.netlify.app/scriptsStoneCalc.js

var metroIndexMap = map[string]string{
	"T4L":      "00",
	"T4E":      "01",
	"T4R":      "02",
	"T4C":      "03",
	"T3E":      "04",
	"3 Slot":   "05",
	"T3R":      "06",
	"T3C":      "07",
	"T2R":      "08",
	"T2C":      "09",
	"T1C":      "10",
	"T4L SIAB": "11",
	"T4E SIAB": "12",
	"T4R SIAB": "13",
	"Empty":    "14",
}

var compassIndexMap = map[string]string{
	"T4L":      "00",
	"T4E":      "01",
	"T4R":      "02",
	"T4C":      "03",
	"T3R":      "04",
	"T3C":      "05",
	"T2C":      "06",
	"T1C":      "07",
	"3 Slot":   "08",
	"T4L SIAB": "09",
	"T4E SIAB": "10",
	"T4R SIAB": "11",
	"Empty":    "12",
}

var gussetIndexMap = map[string]string{
	"T4L":      "00",
	"T4E":      "01",
	"T2E":      "02",
	"3 Slot":   "03",
	"T4C":      "04",
	"T3R":      "05",
	"T3C":      "06",
	"T2C":      "07",
	"T1C":      "08",
	"T4L SIAB": "09",
	"T4E SIAB": "10",
	"T4R SIAB": "11",
	"Empty":    "12",
}

var deflectorIndexMap = map[string]string{
	"T4L":    "00",
	"T4E":    "01",
	"T4R":    "02",
	"T4C":    "03",
	"T3R":    "04",
	"T3C":    "05",
	"T2C":    "06",
	"T1C":    "07",
	"3 Slot": "08",
	"Empty":  "09",
}

var base62 = struct {
	charset []rune
}{
	charset: []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"),
}

func encode(integer uint64) string {
	if integer == 0 {
		return "0"
	}
	s := []rune{}
	for integer > 0 {
		s = append([]rune{base62.charset[integer%62]}, s...)
		integer /= 62
	}
	return string(s)
}

func decode(chars string) uint64 {
	var result uint64
	for i, c := range chars {
		result += uint64(strings.IndexRune(string(base62.charset), c)) * uint64(math.Pow(62, float64(len(chars)-i-1)))
	}
	return result
}

// Chunk16 converts a string of numbers into chunks of length 16, and encodes each using base62.
func Chunk16(x string) string {
	dataEncoded62 := ""
	if len(x) < 16 {
		return encode(uint64(parseInt(x)))
	}

	n := len(x) / 15
	for i := 0; i < n; i++ {
		tmp := "8" + x[i*15:15+i*15]
		dataEncoded62 += encode(uint64(parseInt(tmp)))
	}
	tmp := x[n*15:]
	dataEncoded62 += encode(uint64(parseInt("8" + tmp)))
	return dataEncoded62
}

// Unchunk16 does the exact opposite of Chunk16.
func Unchunk16(dataEncoded62 string) string {
	x := ""
	inputLength := 9
	if len(dataEncoded62) < 10 {
		return strconv.FormatUint(decode(dataEncoded62), 10)
	}

	n := len(dataEncoded62) / inputLength
	for i := 0; i < n; i++ {
		x += strconv.FormatUint(decode(dataEncoded62[i*inputLength:inputLength+i*inputLength]), 10)[1:]
	}
	tmp := dataEncoded62[n*inputLength:]
	x += strconv.FormatUint(decode(tmp), 10)[1:]
	return x
}

// parseInt converts a string to an integer.
func parseInt(s string) uint64 {
	num, _ := strconv.ParseUint(s, 10, 64)
	return num
}

// GetStaabmiaLink returns a link to the Staabia calculator.
func GetStaabmiaLink(darkMode bool, modifierType ei.GameModifier_GameDimension, modifierMult float64, coopDeflectorBonus int, deflectorSlot string, metroSlot string, compassSlot string, gussetSlot string, shippingRate float64) string {
	link := "https://srsandbox-staabmia.netlify.app/stone-calc?data="
	version := "v-4"
	itemsToSweep := []string{"Padding", "DarkMode", "Metro", "Comp", "Gusset", "Defl", "ShipColleggtibles", "ShipColleggtibles2", "Modifiers", "DeflectorSelect"}
	itemsData := make([]string, len(itemsToSweep))

	// Build Base64 data
	base64res := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%g-%d", modifierMult, coopDeflectorBonus)))
	base64encoded := strings.Split(base64res, "=")[0]

	// Build Base 62 data
	// Padding for first element
	itemsData[0] = "1" // Prefix Padding
	itemsData[1] = "0" // Default to Light Mode
	if darkMode {
		itemsData[1] = "1" // Dark Mode
	}

	itemsData[2] = metroIndexMap[metroSlot]         // Metro
	itemsData[3] = compassIndexMap[compassSlot]     // Comp
	itemsData[4] = gussetIndexMap[gussetSlot]       // Gusset
	itemsData[5] = deflectorIndexMap[deflectorSlot] // Defl

	// Determine the combination of multipliers for shippingRate
	multipliers := []float64{1.0, 1.01, 1.02, 1.03, 1.05}
	multiplierMap := map[float64]string{
		1.0:  "05",
		1.01: "04",
		1.02: "03",
		1.03: "02",
		1.05: "00",
	}

	for i := 0; i < len(multipliers); i++ {
		if itemsData[6] != "" {
			break
		}
		for j := 0; j < len(multipliers); j++ {
			fmt.Printf("%f %f %f %f\n", multipliers[i], multipliers[j], (multipliers[i] * multipliers[j]), shippingRate)
			if math.Abs(multipliers[i]*multipliers[j]-shippingRate) < 1e-9 {
				itemsData[6] = multiplierMap[multipliers[i]] // ShipColleggtibles
				itemsData[7] = multiplierMap[multipliers[j]] // ShipColleggtibles2
				break
			}
		}
	}

	switch modifierType {
	case ei.GameModifier_EGG_LAYING_RATE:
		itemsData[8] = "01"
	case ei.GameModifier_SHIPPING_CAPACITY:
		itemsData[8] = "02"
	case ei.GameModifier_HAB_CAPACITY:
		itemsData[8] = "00"
	}
	itemsData[9] = "01" // DeflectorSelect for All Deflectors
	base62encoded := Chunk16(strings.Join(itemsData, ""))

	return link + version + base64encoded + "=" + base62encoded
}
