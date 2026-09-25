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

// determineFarmIcons returns the icon to use for Home and Virtue farms based on the player's active farm.
func determineFarmIcons(backup *ei.Backup) (homeIcon string, virtueIcon string) {
	homeIcon = "🏠"
	virtueIcon = "🕊️"

	currentEgg := ei.Egg_UNKNOWN
	for _, f := range backup.GetFarms() {
		if f.GetFarmType() == ei.FarmType_HOME {
			currentEgg = f.GetEggType()
			break
		}
	}
	if currentEgg == ei.Egg_UNKNOWN && len(backup.GetFarms()) > 0 {
		currentEgg = backup.GetFarms()[0].GetEggType()
	}
	if currentEgg == ei.Egg_UNKNOWN && backup.GetSim() != nil {
		currentEgg = backup.GetSim().GetEggType()
	}

	onVirtueFarm := currentEgg >= ei.Egg_CURIOSITY && currentEgg <= ei.Egg_KINDNESS

	if onVirtueFarm {
		// If the player is on a virtue farm then the home farm should show the enlightenment egg
		if emoji, ok := ei.GetEggEmojiMarkdownIfExists(ei.Egg_ENLIGHTENMENT); ok {
			homeIcon = emoji
		}
		// If the player's Home or Virtue farm is on an egg, use the emoji for that egg if it's available.
		if emoji, ok := ei.GetEggEmojiMarkdownIfExists(currentEgg); ok {
			virtueIcon = emoji
		}
	} else {
		// If the player's Home or Virtue farm is on an egg, use the emoji for that egg if it's available.
		if currentEgg != ei.Egg_UNKNOWN {
			if emoji, ok := ei.GetEggEmojiMarkdownIfExists(currentEgg); ok {
				homeIcon = emoji
			}
		}
		// If the player isn't in the virtue farm then the icon should be the TE icon
		if emoji, ok := ei.GetBotEmojiMarkdownIfExists("egg_truth"); ok {
			virtueIcon = emoji
		}
	}

	return homeIcon, virtueIcon
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

	inUseSlots := 0
	if afxDB := backup.GetArtifactsDb(); afxDB != nil {
		if virtueDB := afxDB.GetVirtueAfxDb(); virtueDB != nil {
			if activeAfx := virtueDB.GetActiveArtifacts(); activeAfx != nil {
				for _, slot := range activeAfx.GetSlots() {
					if slot != nil && slot.GetOccupied() {
						inUseSlots++
					}
				}
			}
		}
	}
	maxCTEResult := ei.CalculateMaxClothedTEWithSlotHint(backup, inUseSlots)
	cte := maxCTEResult.ClothedTE
	if cte < 0 {
		cte = 0
	}
	pendingCTE := cte + float64(pendingTE)

	// Home Farm with pending TE
	homeNakedEBPending := ei.GetEarningsBonus(backup, float64(earnedTE+pendingTE))
	homeNakedRolePending := ei.EarningBonusPercentToFarmerRole(homeNakedEBPending)
	homeDressedEBPending := ei.GetDressedEarningsBonus(backup, float64(earnedTE+pendingTE))
	homeDressedRolePending := ei.EarningBonusPercentToFarmerRole(homeDressedEBPending)

	homeIcon, virtueIcon := determineFarmIcons(backup)

	var desc strings.Builder
	primaryColor := parseHexColor(homeDressedRole.Color)

	fmtFmt := map[string]any{"decimals": 3, "trim": true}

	switch farmChoice {
	case FarmHome:
		fmt.Fprintf(&desc, "### %s Home Farm\n", homeIcon)
		fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d\n",
			pe,
			ei.FormatEIValue(se, fmtFmt),
			earnedTE,
		)
		fmt.Fprintf(&desc, "**Nekkid EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(homeNakedEB, fmtFmt),
			homeNakedRole.Name,
		)
		fmt.Fprintf(&desc, "**Dressed EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(homeDressedEB, fmtFmt),
			homeDressedRole.Name,
		)

	case FarmVirtue:
		primaryColor = parseHexColor(virtueRole.Color)
		fmt.Fprintf(&desc, "### %s Virtue Farm\n", virtueIcon)
		if pendingTE > 0 {
			fmt.Fprintf(&desc, "**TE**: %d (+%d pending) · **CTE**: %.0f · **Pending CTE**: %.0f\n", earnedTE, pendingTE, cte, pendingCTE)
		} else {
			fmt.Fprintf(&desc, "**TE**: %d · **CTE**: %.0f\n", earnedTE, cte)
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
			fmt.Fprintf(&desc, "### %s Home Farm (with pending TE)\n", homeIcon)
			fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d (%d + %d pending)\n",
				pe,
				ei.FormatEIValue(se, fmtFmt),
				earnedTE+pendingTE,
				earnedTE,
				pendingTE,
			)
			fmt.Fprintf(&desc, "**Nekkid EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(homeNakedEBPending, fmtFmt),
				homeNakedRolePending.Name,
			)
			fmt.Fprintf(&desc, "**Dressed EB**: %s%% · **Role**: %s\n",
				ei.FormatEIValue(homeDressedEBPending, fmtFmt),
				homeDressedRolePending.Name,
			)
		}

	default: // FarmHomeAndVirtue
		fmt.Fprintf(&desc, "### %s Home Farm\n", homeIcon)
		fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d\n",
			pe,
			ei.FormatEIValue(se, fmtFmt),
			earnedTE,
		)
		fmt.Fprintf(&desc, "**Nekkid EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(homeNakedEB, fmtFmt),
			homeNakedRole.Name,
		)
		fmt.Fprintf(&desc, "**Dressed EB**: %s%% · **Role**: %s\n",
			ei.FormatEIValue(homeDressedEB, fmtFmt),
			homeDressedRole.Name,
		)

		fmt.Fprintf(&desc, "### %s Virtue Farm\n", virtueIcon)
		if pendingTE > 0 {
			fmt.Fprintf(&desc, "**TE**: %d (+%d pending) · **CTE**: %.0f · **Pending CTE**: %.0f\n", earnedTE, pendingTE, cte, pendingCTE)
		} else {
			fmt.Fprintf(&desc, "**TE**: %d · **CTE**: %.0f\n", earnedTE, cte)
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
			fmt.Fprintf(&desc, "### %s Home Farm (with pending TE)\n", homeIcon)
			fmt.Fprintf(&desc, "**PE**: %d · **SE**: %s · **TE**: %d (%d + %d pending)\n",
				pe,
				ei.FormatEIValue(se, fmtFmt),
				earnedTE+pendingTE,
				earnedTE,
				pendingTE,
			)
			fmt.Fprintf(&desc, "**Nekkid EB**: %s%% · **Role**: %s\n",
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
