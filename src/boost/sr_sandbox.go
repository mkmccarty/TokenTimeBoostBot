package boost

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// playerData stores player-related data.
/*
type playerData struct {
	name         string // Player name
	tokens       string // Player tokens used
	te           string // Truth Egg (TE) count
	mirror       bool   // True if mirror (+1 token)
	colleggtible bool   // True if player has all collectibles
	sink         bool   // True if sink (no tval)
	creator      bool   // True if creator (no join delay)
	item1        string // Item slot 1
	item2        string // Item slot 2
	item3        string // Item slot 3
	item4        string // Item slot 4
	item5        string // Item slot 5
	item6        string // Item slot 6
	item7        string // Item slot 7
	item8        string // Item slot 8
}

// inputData stores full coop info.
type inputData struct {
	crtToggle   bool         // Chicken Runs Maxed?*
	tokenToggle bool         // Token Value Maxed?*
	ggToggle    bool         // Generous Gifts?
	eggUnit     int          // egg unit index (default: q)
	durUnit     int          // duration unit index (default: days)
	modName     int          // contract modifier type index
	cxpToggle   bool         // False = Legacy, True = Seasonal Run
	crtTime     string       // Join Delay (seconds)
	mpft        string       // Minutes/TokenGift/Player*
	duration    string       // Duration (days)
	targetEgg   string       // Target Egg Amount (q)
	tokenTimer  string       // Token Timer (minutes)
	modifiers   string       // Modifier Multiplier
	numPlayers  int          // CoopSize:
	btvTarget   string       // BTV/complTime Target
	players     []playerData // Player list for results
}
*/

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

	switch {
	case v >= 1e18:
		str, unit = fmt.Sprintf("%g", v/1e18), 0 // Quintillion
	case v >= 1e15:
		str, unit = fmt.Sprintf("%g", v/1e15), 1 // quadrillion
	default:
		str, unit = fmt.Sprintf("%g", v/1e12), 2 // Trillion
	}

	// Apply SR sandbox ordering: 0=quadrillion, 1=Quintillion, 2=Trillion
	if srSandboxOrdering {
		switch unit {
		case 0: // Quintillion -> 1
			unit = 1
		case 1: // quadrillion -> 0
			unit = 0
		case 2: // Trillion <-> 2
			unit = 2
		}
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
	// HabCap (🏠)
	case c.ModifierHabCap != 1.0 && c.ModifierHabCap > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierHabCap), 0
	// IHR (🐣)
	case c.ModifierIHR != 1.0 && c.ModifierIHR > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierIHR), 1
	// SR (🛻)
	case c.ModifierSR != 1.0 && c.ModifierSR > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierSR), 2
	// ELR (🥚)
	case c.ModifierELR != 1.0 && c.ModifierELR > 0.0:
		return fmt.Sprintf("%1.3g", c.ModifierELR), 3
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

func base62Encode(integer int) string {
	if integer == 0 {
		return "0"
	}
	s := []rune{}
	for integer > 0 {
		remainder := integer % 62
		s = append([]rune{base62Charset[remainder]}, s...)
		integer = integer / 62
	}
	return string(s)
}

func dataToBase64(data, data2 string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(data))
	return encoded + "=" + data2
}

func formatCompactTargetEgg(targetEgg float64) (string, int) {
	switch {
	case targetEgg >= 1e18:
		qVal := targetEgg / 1e18
		return strings.ReplaceAll(fmt.Sprintf("%.1f", qVal), ".", "p"), 1
	case targetEgg >= 1e15:
		qVal := int64(math.Floor(targetEgg / 1e15))
		return strconv.FormatInt(qVal, 10), 0
	default:
		tVal := int64(math.Floor(targetEgg / 1e12))
		return strconv.FormatInt(tVal, 10), 2
	}
}

func fmtActiveModifierCompact(c *ei.EggIncContract) (string, int) {
	if c == nil {
		return "1", 0
	}

	value, idx := FmtActiveModifier(c)
	return strings.ReplaceAll(value, ".", "p"), idx
}

func chunk16(x string) string {
	if len(x) < 16 {
		i, _ := strconv.Atoi(x)
		return base62Encode(i)
	}
	y := ""
	n := len(x) / 15
	for i := 0; i < n; i++ {
		tmp := "8" + x[i*15:(i*15)+15]
		num, _ := strconv.Atoi(tmp)
		y += base62Encode(num)
	}
	tmp := x[n*15:]
	if tmp != "" {
		num, _ := strconv.Atoi("8" + tmp)
		y += base62Encode(num)
	}
	return y
}

/*
func gatherData(input inputData) (string, string, error) {
	if input.numPlayers != len(input.players) {
		return "", "", errors.New("numPlayers does not match player array length")
	}

	SEPARATOR := "-"
	data := []string{}
	singleStr := []string{}

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
		data = append(data, p.name, p.tokens, p.te)

		singleStr2 = append(singleStr2,
			convertBool(p.mirror),
			convertBool(p.colleggtible),
			convertBool(p.sink),
			convertBool(p.creator),
			p.item1, p.item2, p.item3, p.item4,
			p.item5, p.item6, p.item7, p.item8,
		)
	}

	data2 := []string{chunk16(strings.Join(singleStr2, ""))}

	return strings.Join(data, SEPARATOR), strings.Join(data2, SEPARATOR), nil
}
*/

// EncodeData generates and encodes configuration data for the SR Sandbox v-5.
//
// Parameters:
//   - CxpToggle (bool): enables or disables CXP calculations.
//   - TargetEgg (string): target egg count.
//   - TokenTimer (string): token timer interval in minutes.
//   - contractLengthInSeconds (int): total contract duration in seconds.
//   - NumPlayers (int): number of players.
//   - c (*ei.EggIncContract): pointer to the contract data.
//
// Returns:
//   - (string): version-tagged encoded SR Sandbox data.
//   - (error): returned if encoding fails.
func EncodeData(cxpToggle bool, targetEgg float64, tokenTimer string, contractLengthInSeconds, numPlayers int, c *ei.EggIncContract) (string, error) {

	// Duration formatting
	contractDuration := time.Duration(contractLengthInSeconds) * time.Second
	durStr, durUnit := bottools.FmtDurationSingleUnit(contractDuration)

	// Keep GG behavior aligned with the previous encoder.
	ggMultiplier, ultraGGMultiplier, _ := ei.GetGenerousGiftEvent()
	ggToggle := ggMultiplier > 1.0 || ultraGGMultiplier > 1.0

	goalStr, eggUnit := formatCompactTargetEgg(targetEgg)
	modifierValue, modName := fmtActiveModifierCompact(c)

	globalSettings := fmt.Sprintf("11%s%d%d%d%s", convertBool(ggToggle), eggUnit, durUnit, modName, convertBool(cxpToggle))

	data := []string{
		"BBPlayer|1",
		globalSettings,
		"20", // Join delay in seconds
		"11", // Minutes per token gift per player
		durStr,
		goalStr,
		tokenTimer,
		modifierValue,
		strconv.Itoa(numPlayers),
		"2",
		strconv.Itoa(numPlayers), // Group count for non-sink players
		"5",
		"50",
	}

	playerPattern := []string{
		"1",
		convertBool(false),
		convertBool(true),
		convertBool(false),
		convertBool(false),
		"00", "00", "00", "00", "00", "00", "00", "00",
	}

	hashPart := chunk16(strings.Join(playerPattern, ""))
	encoded := dataToBase64(strings.Join(data, "-"), hashPart)

	contractName := ""
	if c != nil {
		contractName = c.Name
		if c.ID != "" {
			contractName = fmt.Sprintf("%s (%s)", c.Name, c.ID)
		}
	}
	contractNameB64 := base64.StdEncoding.EncodeToString([]byte(contractName))

	final := "v_5" + encoded + "&c=" + contractNameB64
	return final, nil
}
