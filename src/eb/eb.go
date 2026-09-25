package eb

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// decryptEggIncID decrypts an encrypted Egg Inc ID string using config.Key.
func decryptEggIncID(eiID string) string {
	if eiID == "" {
		return ""
	}
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		return ""
	}
	decodedData, err := base64.StdEncoding.DecodeString(eiID)
	if err != nil {
		return ""
	}
	decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
	if err != nil {
		return ""
	}
	id := string(decryptedData)
	if len(id) == 18 && id[:2] == "EI" {
		return id
	}
	return ""
}

// getEggIncID retrieves and decrypts the user's stored Egg Inc ID.
func getEggIncID(userID string) string {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	return decryptEggIncID(eiID)
}

// parseHexColor converts a hex string (e.g., "#46a8eb") to an integer color.
func parseHexColor(hex string) int {
	hex = strings.TrimPrefix(hex, "#")
	if val, err := strconv.ParseInt(hex, 16, 32); err == nil {
		return int(val)
	}
	return 0x888888
}

// ExecuteEb executes the EB display logic and responds to the interaction event.
func ExecuteEb(e dc.InteractionEvent, farmChoice string, eggIncID string, okayToSave bool) {
	userID := e.UserID()

	ephemeral := e.ChannelID() == "571836573243539476" // ACO- #bot-commands
	_ = e.Defer(ephemeral)

	backup, _ := ei.GetFirstContactFromAPI(eggIncID, userID, okayToSave)
	if backup == nil {
		_ = e.Followup(dc.Message{
			Content:   "Unable to retrieve game data for this Egg Inc ID. Please verify your ID and try again.",
			Ephemeral: true,
		})
		return
	}

	embed := BuildEbEmbed(backup, farmChoice, userID)

	_ = e.Followup(dc.Message{
		Ephemeral: ephemeral,
		Embeds:    []dc.Embed{embed},
	})
}

// BuildEbEmbed computes the earnings bonus values and generates the Discord embed.
func BuildEbEmbed(backup *ei.Backup, farmChoice string, userID string) dc.Embed {
	game := backup.GetGame()
	pe := game.GetEggsOfProphecy()
	se := game.GetSoulEggsD()

	userName := ei.NormalizePlayerNameForDisplay(backup.GetUserName())
	if userName == "" {
		userName = "Farmer"
	}

	var earnedTE uint32
	var pendingTE uint32

	virtue := backup.GetVirtue()
	if virtue != nil {
		eovEarnedList := virtue.GetEovEarned()
		deliveredList := virtue.GetEggsDelivered()
		for i := 0; i < 5 && i < len(eovEarnedList) && i < len(deliveredList); i++ {
			eov := eovEarnedList[i]
			delivered := deliveredList[i]

			eovEarned := ei.CountTruthEggTiersPassed(delivered)
			eovPending := ei.PendingTruthEggs(delivered, eov)

			earnedTE += max(eovEarned-eovPending, 0)
			pendingTE += eovPending
		}
	}

	if userID != "" {
		farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", earnedTE))
	}

	// Home Farm EB calculations (using earned TE, not pending)
	homeNakedEB := ei.GetEarningsBonus(backup, float64(earnedTE))
	homeNakedRole := ei.EarningBonusPercentToFarmerRole(homeNakedEB)
	homeDressedEB := ei.GetDressedEarningsBonus(backup, float64(earnedTE))
	homeDressedRole := ei.EarningBonusPercentToFarmerRole(homeDressedEB)

	// Virtue Farm EB calculations: EB is 1.10^TE
	virtueRatio := math.Pow(1.10, float64(earnedTE))
	virtueEBPercent := virtueRatio * 100.0
	virtueRole := ei.EarningBonusToFarmerRole(virtueRatio)

	virtueRatioPending := math.Pow(1.10, float64(earnedTE+pendingTE))
	virtueEBPercentPending := virtueRatioPending * 100.0
	virtueRolePending := ei.EarningBonusToFarmerRole(virtueRatioPending)

	// Home Farm with pending TE
	homeNakedEBPending := ei.GetEarningsBonus(backup, float64(earnedTE+pendingTE))
	homeNakedRolePending := ei.EarningBonusPercentToFarmerRole(homeNakedEBPending)
	homeDressedEBPending := ei.GetDressedEarningsBonus(backup, float64(earnedTE+pendingTE))
	homeDressedRolePending := ei.EarningBonusPercentToFarmerRole(homeDressedEBPending)

	var desc strings.Builder
	primaryColor := parseHexColor(homeDressedRole.Color)

	fmtFmt := map[string]any{"decimals": 3, "trim": true}

	switch farmChoice {
	case FarmHome:
		fmt.Fprintf(&desc, "### 🏠 Home Farm\n")
		fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d\n",
			pe,
			ei.FormatEIValue(se, fmtFmt),
			earnedTE,
		)
		fmt.Fprintf(&desc, "**Naked EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(homeNakedEB, fmtFmt),
			homeNakedRole.Name,
		)
		fmt.Fprintf(&desc, "**Dressed EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(homeDressedEB, fmtFmt),
			homeDressedRole.Name,
		)

	case FarmVirtue:
		primaryColor = parseHexColor(virtueRole.Color)
		fmt.Fprintf(&desc, "### 🕊️ Virtue Farm\n")
		if pendingTE > 0 {
			fmt.Fprintf(&desc, "**TE**: %d (+%d pending)\n", earnedTE, pendingTE)
		} else {
			fmt.Fprintf(&desc, "**TE**: %d\n", earnedTE)
		}
		fmt.Fprintf(&desc, "**Actual EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(virtueEBPercent, fmtFmt),
			virtueRole.Name,
		)
		if pendingTE > 0 {
			fmt.Fprintf(&desc, "**Pending EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(virtueEBPercentPending, fmtFmt),
				virtueRolePending.Name,
			)
		}
		desc.WriteString("-# In Virtue farms, PE and SE do not count (EB is 1.10^TE)\n")

		if pendingTE > 0 {
			desc.WriteString("\n### 🏠 Home Farm (with pending TE)\n")
			fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d (%d + %d pending)\n",
				pe,
				ei.FormatEIValue(se, fmtFmt),
				earnedTE+pendingTE,
				earnedTE,
				pendingTE,
			)
			fmt.Fprintf(&desc, "**Naked EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(homeNakedEBPending, fmtFmt),
				homeNakedRolePending.Name,
			)
			fmt.Fprintf(&desc, "**Dressed EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(homeDressedEBPending, fmtFmt),
				homeDressedRolePending.Name,
			)
		}

	default: // FarmHomeAndVirtue
		fmt.Fprintf(&desc, "### 🏠 Home Farm\n")
		fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d\n",
			pe,
			ei.FormatEIValue(se, fmtFmt),
			earnedTE,
		)
		fmt.Fprintf(&desc, "**Naked EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(homeNakedEB, fmtFmt),
			homeNakedRole.Name,
		)
		fmt.Fprintf(&desc, "**Dressed EB**: %s%% · **Role**: %s\n\n",
			ei.FormatEIValue(homeDressedEB, fmtFmt),
			homeDressedRole.Name,
		)

		fmt.Fprintf(&desc, "### 🕊️ Virtue Farm\n")
		if pendingTE > 0 {
			fmt.Fprintf(&desc, "**TE**: %d (+%d pending)\n", earnedTE, pendingTE)
		} else {
			fmt.Fprintf(&desc, "**TE**: %d\n", earnedTE)
		}
		fmt.Fprintf(&desc, "**Actual EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(virtueEBPercent, fmtFmt),
			virtueRole.Name,
		)
		if pendingTE > 0 {
			fmt.Fprintf(&desc, "**Pending EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(virtueEBPercentPending, fmtFmt),
				virtueRolePending.Name,
			)
		}
		desc.WriteString("-# In Virtue farms, PE and SE do not count (EB is 1.10^TE)\n")

		if pendingTE > 0 {
			desc.WriteString("\n### 🏠 Home Farm (with pending TE)\n")
			fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d (%d + %d pending)\n",
				pe,
				ei.FormatEIValue(se, fmtFmt),
				earnedTE+pendingTE,
				earnedTE,
				pendingTE,
			)
			fmt.Fprintf(&desc, "**Naked EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(homeNakedEBPending, fmtFmt),
				homeNakedRolePending.Name,
			)
			fmt.Fprintf(&desc, "**Dressed EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(homeDressedEBPending, fmtFmt),
				homeDressedRolePending.Name,
			)
		}

	}

	return dc.Embed{
		Title:       fmt.Sprintf("Earnings Bonus — %s", userName),
		Description: strings.TrimSpace(desc.String()),
		Color:       primaryColor,
	}
}
