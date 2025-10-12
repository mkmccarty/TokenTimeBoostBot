package boost

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
)

// playerData stores player-related data.
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

// EncodeData generates and encodes configuration data for the SR Sandbox v-5.
//
// Parameters:
//   - CxpToggle (bool): enables or disables CXP calculations.
//   - TargetEgg (string): target egg count.
//   - TokenTimer (string): token timer interval in minutes.
//   - Modifiers (string): contract modifiers (TODO: implement support).
//   - contractLengthInSeconds (int): total contract duration in seconds.
//   - NumPlayers (int): number of players.
//
// Returns:
//   - (string): version-tagged encoded SR Sandbox data.
//   - (error): returned if encoding fails.
func EncodeData(cxpToggle bool, targetEgg, tokenTimer, modifiers string, contractLengthInSeconds, numPlayers int) (string, error) {

	// Duiration formatting
	contractDuration := time.Duration(contractLengthInSeconds) * time.Second
	durStr, durUnit := bottools.FmtDurationSingleUnit(contractDuration)

	input := inputData{
		crtToggle:   true,
		tokenToggle: true,
		ggToggle:    false,
		eggUnit:     2,       // in order q, Q, T
		durUnit:     durUnit, // in order days, hours, minutes, seconds
		modName:     0,       // TODO implement support
		cxpToggle:   cxpToggle,
		crtTime:     "20", // seconds
		mpft:        "10", // minutes
		duration:    durStr,
		targetEgg:   targetEgg,
		tokenTimer:  tokenTimer, // minutes
		modifiers:   modifiers,  // TODO implement support
		numPlayers:  numPlayers,
		btvTarget:   "2", // old max buff
	}

	tokensArr := make([]string, input.numPlayers)
	teArr := make([]string, input.numPlayers)

	for i := 0; i < input.numPlayers; i++ {
		tokensArr[i] = "6"
		teArr[i] = "10"
	}

	// Full leggy assumption
	input.players = make([]playerData, input.numPlayers)
	for i := 0; i < input.numPlayers; i++ {
		input.players[i] = playerData{
			name:         fmt.Sprintf("BBPlayer%d", i+1),
			tokens:       tokensArr[i],
			te:           teArr[i],
			mirror:       false,
			colleggtible: true,
			sink:         false,
			creator:      false,
			item1:        "00",
			item2:        "00",
			item3:        "00",
			item4:        "00",
			item5:        "00",
			item6:        "00",
			item7:        "00",
			item8:        "00",
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
