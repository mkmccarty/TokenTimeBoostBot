package boost

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// playerData stores player-related data.
type playerData struct {
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

// inputData stores full coop info
type inputData struct {
	CrtToggle   bool         // Chicken Runs Maxed?*
	TokenToggle bool         // Token Value Maxed?*
	GGToggle    bool         // Generous Gifts?
	EggUnit     int          // egg unit index (default: q)
	DurUnit     int          // duration unit index (default: days)
	ModName     int          // contract modifier type index
	CxpToggle   bool         // False = Legacy, True = Seasonal Run
	CrtTime     string       // Join Delay (seconds)
	Mpft        string       // Minutes/TokenGift/Player*
	Duration    string       // Duration (days)
	TargetEgg   string       // Target Egg Amount (q)
	TokenTimer  string       // Token Timer (minutes)
	Modifiers   string       // Modifier Multiplier
	NumPlayers  int          // CoopSize:
	BtvTarget   string       // BTV/complTime Target
	Players     []playerData // Player list for results
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
	encoded := base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(data)))
	parts := strings.Split(encoded, "=")
	dataB64 := parts[0]
	dataEncoded := dataB64 + "=" + data2
	return dataEncoded
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

func gatherData(input inputData) (string, string, error) {
	if input.NumPlayers != len(input.Players) {
		return "", "", errors.New("numPlayers does not match player array length")
	}

	SEPARATOR := "-"
	data := []string{}
	singleStr := []string{}

	singleStr = append(singleStr,
		convertBool(input.CrtToggle),
		convertBool(input.TokenToggle),
		convertBool(input.GGToggle),
		strconv.Itoa(input.EggUnit),
		strconv.Itoa(input.DurUnit),
		strconv.Itoa(input.ModName),
		convertBool(input.CxpToggle),
	)
	data = append(data, strings.Join(singleStr, ""))

	data = append(data,
		input.CrtTime,
		input.Mpft,
		input.Duration,
		input.TargetEgg,
		input.TokenTimer,
		input.Modifiers,
		strconv.Itoa(input.NumPlayers),
		input.BtvTarget,
	)

	singleStr2 := []string{"1"} // Start with leading 1

	for _, p := range input.Players {
		data = append(data, p.Name, p.Tokens, p.TE)

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

// EncodeData generates and encodes configuration data for the SR Sandbox v-5
// Parameters:
//   - CxpToggle (bool): enables or disables CXP calculations.
//   - Duration (string): total duration in days.
//   - TargetEgg (string): target egg count.
//   - TokenTimer (string): token timer interval in minutes.
//   - Modifiers (string): TODO contract modifiers
//   - NumPlayers (int): number of players.
//
// Returns:
//   - (string): version-tagged encoded SR Sandbox data.
//   - (error): returned if encoding fails.
func EncodeData(CxpToggle bool, Duration, TargetEgg, TokenTimer, Modifiers string, NumPlayers int) (string, error) {
	input := inputData{
		CrtToggle:   true,
		TokenToggle: true,
		GGToggle:    false,
		EggUnit:     0,
		DurUnit:     0,
		ModName:     0, // TODO implement support
		CxpToggle:   CxpToggle,
		CrtTime:     "20",     // seconds
		Mpft:        "10",     // minutes
		Duration:    Duration, // days
		TargetEgg:   TargetEgg,
		TokenTimer:  TokenTimer, // minutes
		Modifiers:   Modifiers,  // TODO implement support
		NumPlayers:  NumPlayers,
		BtvTarget:   "2", // old max buff
	}

	tokensArr := make([]string, input.NumPlayers)
	teArr := make([]string, input.NumPlayers)

	for i := 0; i < input.NumPlayers; i++ {
		tokensArr[i] = "6"
		teArr[i] = "10"
	}

	// Full leggy assumption
	input.Players = make([]playerData, input.NumPlayers)
	for i := 0; i < input.NumPlayers; i++ {
		input.Players[i] = playerData{
			Name:         fmt.Sprintf("BBPlayer%d", i+1),
			Tokens:       tokensArr[i],
			TE:           teArr[i],
			Mirror:       false,
			Colleggtible: true,
			Sink:         false,
			Creator:      false,
			Item1:        "00",
			Item2:        "00",
			Item3:        "00",
			Item4:        "00",
			Item5:        "00",
			Item6:        "00",
			Item7:        "00",
			Item8:        "00",
		}
	}

	d1, d2, err := gatherData(input)
	if err != nil {
		return "", err
	}

	encoded := dataToBase64(d1, d2)
	final := "v-5" + encoded
	return final, nil
}
