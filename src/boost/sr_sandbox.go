package boost

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// SandboxPlayer stores player-related data.
type SandboxPlayer struct {
	Name         string // Player name
	Tokens       string // Player tokens used
	TE           string // Truth Egg (TE) count
	Mirror       bool   // True if mirror (+1 token)
	Colleggtible bool   // True if player has all collectibles
	Sink         bool   // True if sink (no tval)
	Creator      bool   // True if creator (no join delay)
	Item1        string // Item slot 1
	Item2        string // Item slot 2
	Item3        string // Item slot 3
	Item4        string // Item slot 4
	Item5        string // Item slot 5
	Item6        string // Item slot 6
	Item7        string // Item slot 7
	Item8        string // Item slot 8
}

// inputData stores full coop info.
type inputData struct {
	crtToggle   bool            // Chicken Runs Maxed?*
	tokenToggle bool            // Token Value Maxed?*
	ggToggle    bool            // Generous Gifts?
	eggUnit     int             // egg unit index (default: q)
	durUnit     int             // duration unit index (default: days)
	modName     int             // contract modifier type index
	cxpToggle   bool            // False = Legacy, True = Seasonal Run
	crtTime     string          // Join Delay (seconds)
	mpft        string          // Minutes/TokenGift/Player*
	duration    string          // Duration (days)
	targetEgg   string          // Target Egg Amount (q)
	tokenTimer  string          // Token Timer (minutes)
	modifiers   string          // Modifier Multiplier
	numPlayers  int             // CoopSize:
	btvTarget   string          // BTV/complTime Target
	players     []SandboxPlayer // Player list for results
}

// FmtNumberSingleUnit converts a float64 to a string with the correct unit scale.
// Parameters:
//   - v (float64): numeric value
//   - srSandboxOrdering (bool): optional flag to use SR sandbox ordering.
//
// Returns:
//   - string: formatted number as a string
//   - int: unit index
//     Normal:     0=Quintillion, 1=quadrillion, 2=Trillion
//     SR Sandbox: 0=quadrillion, 1=Quintillion, 2=Trillion
//     -1=none
func FmtNumberSingleUnit(v float64, srSandboxOrdering bool) (string, int) {
	var str string
	var unit int
	var val float64

	switch {
	case v >= 1e18:
		val, unit = v/1e18, 0 // Quintillion
	case v >= 1e15:
		val, unit = v/1e15, 1 // quadrillion
	default:
		val, unit = v/1e12, 2 // Trillion
	}

	// Apply SR sandbox ordering: 0=quadrillion, 1=Quintillion, 2=Trillion
	if srSandboxOrdering {
		switch unit {
		case 0: // Quintillion -> 1
			unit = 1
			str = fmt.Sprintf("%.1f", val)
			str = strings.ReplaceAll(str, ".", "p")
		case 1: // quadrillion -> 0
			unit = 0
			str = fmt.Sprintf("%.0f", math.Floor(val))
		case 2: // Trillion <-> 2
			unit = 2
			str = fmt.Sprintf("%.0f", math.Floor(val))
		}
	} else {
		str = ei.FormatEIValue(val, map[string]any{"no_scale": true, "no_suffix": true, "max_length": 5, "trim": true, "decimals": 4})
	}

	return str, unit
}

// FmtActiveModifier checks the contract modifiers in order and returns the first active one.
//
// Parameters:
//   - c (*ei.EggIncContract): pointer to the contract data
//
// Returns:
//   - string: formatted active modifier value (e.g., "1.05").
//   - int: index of the active modifier (0: HabCap, 1: IHR, 2: SR, 3: ELR).
func FmtActiveModifier(c *ei.EggIncContract) (string, int) {
	switch {
	// ELR (🥚)
	case c.ModifierELR != 1.0 && c.ModifierELR > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierELR), 3
	// SR (🛻)
	case c.ModifierSR != 1.0 && c.ModifierSR > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierSR), 2
	// IHR (🐣)
	case c.ModifierIHR != 1.0 && c.ModifierIHR > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierIHR), 1
	// HabCap (🏠)
	case c.ModifierHabCap != 1.0 && c.ModifierHabCap > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierHabCap), 0
	// 1x HabCap (🏠)
	default:
		return "1.0", 0
	}
}

func convertBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// Helpers from SR Sandbox
var base62Charset = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func base62Encode(integer uint64) string {
	if integer == 0 {
		return "0"
	}
	s := []rune{}
	for integer > 0 {
		remainder := integer % uint64(62)
		s = append([]rune{base62Charset[remainder]}, s...)
		integer = integer / uint64(62)
	}
	return string(s)
}

func dataToBase64(data, data2 string) string {
	escapedData := strings.ReplaceAll(url.QueryEscape(data), "+", "%20")
	encoded := base64.StdEncoding.EncodeToString([]byte(escapedData))
	parts := strings.Split(encoded, "=")
	dataB64 := parts[0]
	return dataB64 + "=" + data2
}

func chunk16(x string) string {
	if len(x) < 16 {
		i, _ := strconv.ParseUint(x, 10, 64)
		return base62Encode(i)
	}
	y := ""
	n := len(x) / 15
	for i := 0; i < n; i++ {
		tmp := "8" + x[i*15:(i*15)+15]
		num, _ := strconv.ParseUint(tmp, 10, 64)
		y += base62Encode(num)
	}
	tmp := x[n*15:]
	if tmp != "" {
		num, _ := strconv.ParseUint("8"+tmp, 10, 64)
		y += base62Encode(num)
	}
	return y
}

func sanitizeSandboxField(value string) string {
	safeValue := strings.ReplaceAll(value, "|", "")
	safeValue = strings.ReplaceAll(safeValue, "-", "axJEFi")
	return safeValue
}

func gatherData(input inputData, contractName, contractID string) (string, string, error) {

	SEPARATOR := "-"
	data := []string{}
	singleStr := []string{}

	safeContractName := sanitizeSandboxField(contractName)
	safeContractID := sanitizeSandboxField(contractID)
	data = append(data, fmt.Sprintf("%s|%s", safeContractName, safeContractID))

	singleStr = append(singleStr,
		convertBool(input.crtToggle),
		convertBool(input.tokenToggle),
		convertBool(input.ggToggle),
		strconv.Itoa(input.eggUnit),
		strconv.Itoa(input.durUnit),
		strconv.Itoa(input.modName),
		convertBool(input.cxpToggle),
	)
	data = append(data, strings.Join(singleStr, ""))

	data = append(data,
		input.crtTime,
		input.mpft,
		input.duration,
		input.targetEgg,
		input.tokenTimer,
		input.modifiers,
		strconv.Itoa(input.numPlayers),
		input.btvTarget,
	)

	singleStr2 := []string{"1"} // Start with leading 1

	for _, p := range input.players {
		safePlayerName := sanitizeSandboxField(p.Name)
		data = append(data, safePlayerName, p.Tokens, p.TE)

		singleStr2 = append(singleStr2,
			convertBool(p.Mirror),
			convertBool(p.Colleggtible),
			convertBool(p.Sink),
			convertBool(p.Creator),
			p.Item1, p.Item2, p.Item3, p.Item4,
			p.Item5, p.Item6, p.Item7, p.Item8,
		)
	}

	data2 := []string{chunk16(strings.Join(singleStr2, ""))}

	return strings.Join(data, SEPARATOR), strings.Join(data2, SEPARATOR), nil
}

// EncodeSandboxData generates and encodes configuration data for the SR Sandbox v_5 format.
//
// Parameters:
//   - CxpToggle (bool): enables or disables CXP calculations.
//   - TargetEgg (string): target egg count.
//   - TokenTimer (string): token timer interval in minutes.
//   - contractLengthInSeconds (int): total contract duration in seconds.
//   - numPlayers (int): coop size
//   - c (*ei.EggIncContract): pointer to the contract data.
//   - players ([]SandboxPlayer): array of player configurations to include.
//
// Returns:
//   - (string): version-tagged encoded SR Sandbox data.
//   - (error): returned if encoding fails.
func EncodeSandboxData(cxpToggle bool, targetEgg float64, tokenTimer string, contractLengthInSeconds int, numPlayers int, c *ei.EggIncContract, players []SandboxPlayer) (string, error) {

	// Duration formatting
	contractDuration := time.Duration(contractLengthInSeconds) * time.Second
	durStr, durUnit := bottools.FmtDurationSingleUnit(contractDuration)

	// Egg target formatting
	eggStr, eggUnit := FmtNumberSingleUnit(targetEgg, true)

	// Check if Generous Gifts is enabled based on multiplier
	ggMultiplier, ultraGGMultiplier, _ := ei.GetGenerousGiftEvent()
	ggToggle := ggMultiplier > 1.0 || ultraGGMultiplier > 1.0

	// Get modifiers
	modifiers, modName := FmtActiveModifier(c)
	modifiers = strings.ReplaceAll(modifiers, ".", "p")

	btvTarget := "2"
	if cxpToggle {
		btvTarget = "1p875"
	}

	input := inputData{
		crtToggle:   false,
		tokenToggle: true,
		ggToggle:    ggToggle,
		eggUnit:     eggUnit, // in order q, Q, T
		durUnit:     durUnit, // in order days, hours, minutes, seconds
		modName:     modName,
		cxpToggle:   cxpToggle,
		crtTime:     "0",  // seconds
		mpft:        "13", // minutes
		duration:    durStr,
		targetEgg:   eggStr,
		tokenTimer:  tokenTimer, // minutes
		modifiers:   modifiers,
		numPlayers:  numPlayers,
		btvTarget:   btvTarget,
		players:     players,
	}

	d1, d2, err := gatherData(input, c.Name, c.ID)
	if err != nil {
		return "", err
	}

	encoded := dataToBase64(d1, d2)
	finalData := "v_5" + encoded

	b64Name := base64.RawStdEncoding.EncodeToString([]byte(c.Name))
	safeB64Name := url.QueryEscape(b64Name)

	return "data=" + finalData + "&c=" + safeB64Name, nil
}
