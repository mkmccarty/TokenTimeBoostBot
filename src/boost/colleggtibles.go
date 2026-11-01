package boost

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func init() {
	RegisterEggIDModalAction("colleggtibles", func(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
		HandleColleggtiblesModal(e, options, encryptedID, okayToSave)
	})
}

// GetSlashColleggtiblesCommand returns the slash command definition for /colleggtibles.
func GetSlashColleggtiblesCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Show player's colleggtibles and overall buffs by game dimension.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "alt",
			Description:  "Show colleggtibles for a registered alternate account.",
			Required:     false,
			Autocomplete: true,
		},
	}
	if config.IsDevBot() {
		command.Options = append(command.Options, dc.BoolOption{
			Name:        "predict",
			Description: "Show next future predicted drop dates for all colleggtibles (dev only).",
			Required:    false,
		})
	}
	return &command
}

// HandleColleggtiblesAutocomplete handles autocomplete for the alt option.
func HandleColleggtiblesAutocomplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name != "alt" {
		_ = e.RespondChoices(nil)
		return
	}

	effectiveUserID := farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID())
	alts := farmerstate.GetUserAltsWithSavedEID(effectiveUserID)
	if len(alts) == 0 {
		_ = e.RespondChoices(nil)
		return
	}

	value = strings.ToLower(strings.TrimSpace(value))
	choices := make([]dc.Choice[string], 0, len(alts))
	for _, alt := range alts {
		if value != "" {
			matchID := strings.Contains(strings.ToLower(alt.ID), value)
			matchIGN := strings.Contains(strings.ToLower(alt.IGN), value)
			matchDisplay := strings.Contains(strings.ToLower(alt.DisplayName), value)
			if !matchID && !matchIGN && !matchDisplay {
				continue
			}
		}
		displayName := alt.DisplayName
		if len(displayName) > 100 {
			displayName = displayName[:100]
		}
		choices = append(choices, dc.Choice[string]{
			Name:  displayName,
			Value: alt.ID,
		})
		if len(choices) == 25 {
			break
		}
	}

	_ = e.RespondChoices(choices)
}

// getColleggtiblesEggIncID retrieves and decrypts the user's stored Egg Inc ID.
func getColleggtiblesEggIncID(userID string) string {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if eiID == "" && !dc.IsSnowflake(userID) {
		if discordID, err := farmerstate.GetDiscordUserIDFromEiIgnExact(userID); err == nil && discordID != "" {
			eiID = farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
		}
	}
	return ei.DecryptEID(eiID)
}

// HandleColleggtiblesCommand handles the /colleggtibles command.
func HandleColleggtiblesCommand(client dc.Client, e *dc.CommandEvent) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	altParam, _ := e.OptString("alt")
	predict, _ := e.OptBool("predict")
	if !predict {
		predict, _ = e.OptBool("predictions")
	}

	targetID, altEggIncID, notice := farmerstate.ResolveAltSelection(userID, altParam, "Colleggtibles")

	var actAsNotice string
	if isActAs && altParam == "" {
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as", effectiveUserID)
		} else {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as (expires <t:%d:R>)", effectiveUserID, expiresAt.Unix())
		}
	}

	eggIncID := altEggIncID
	if eggIncID == "" {
		eggIncID = getColleggtiblesEggIncID(targetID)
	}

	if eggIncID == "" {
		RequestEggIncIDModal(e, "colleggtibles", e.Options())
		return
	}

	ExecuteColleggtiblesTarget(e, eggIncID, true, targetID, notice, predict, actAsNotice)
}

// HandleColleggtiblesModal handles the modal submission when an Egg Inc ID is provided.
func HandleColleggtiblesModal(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	altParam, _ := options.String("alt")
	predict, _ := options.Bool("predict")
	if !predict {
		predict, _ = options.Bool("predictions")
	}

	targetID, altEggIncID, notice := farmerstate.ResolveAltSelection(userID, altParam, "Colleggtibles")

	var actAsNotice string
	if isActAs && altParam == "" {
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as", effectiveUserID)
		} else {
			actAsNotice = fmt.Sprintf("Acting as <@%s> via /act-as (expires <t:%d:R>)", effectiveUserID, expiresAt.Unix())
		}
	}

	eggIncID := altEggIncID
	if eggIncID == "" {
		eggIncID = ei.DecryptEID(encryptedID)
	}
	if eggIncID == "" {
		_ = e.Respond(dc.Message{
			Content:   "Invalid Egg Inc ID received.",
			Ephemeral: true,
		})
		return
	}

	ExecuteColleggtiblesTarget(e, eggIncID, okayToSave, targetID, notice, predict, actAsNotice)
}

// ExecuteColleggtiblesTarget executes the colleggtibles logic for a target user.
func ExecuteColleggtiblesTarget(e dc.InteractionEvent, eggIncID string, okayToSave bool, targetUserID string, notice string, showAllPredictions bool, actAsNotice ...string) {
	_ = e.Defer(false)

	backup, _ := ei.GetFirstContactFromAPI(eggIncID, targetUserID, okayToSave)
	if backup == nil {
		_ = e.Followup(dc.Message{
			Content: "Unable to retrieve game data for this Egg Inc ID. Please verify your ID and try again.",
		})
		return
	}

	farmerstate.SetFarmerBackupDetails(targetUserID, backup)
	populateColleggtiblesFromBackup(targetUserID, backup)

	avatarURL := ""
	if targetUserID != e.UserID() {
		altDiscordID := farmerstate.FindAltDiscordID(targetUserID, e.UserID())
		if altDiscordID != "" {
			avatarURL = bottools.GetDiscordAvatarURL(e.Client(), e.GuildID(), altDiscordID)
		}
	} else {
		avatarURL = e.AvatarURL()
		if avatarURL == "" && e.Client() != nil && e.GuildID() != "" {
			avatarURL = bottools.GetDiscordAvatarURL(e.Client(), e.GuildID(), e.UserID())
		}
	}

	components, files := BuildColleggtiblesComponents(backup, targetUserID, avatarURL, showAllPredictions)

	if notice != "" {
		components = append([]dc.LayoutComponent{
			dc.TextDisplay{Content: notice},
		}, components...)
	}

	actAsText := ""
	if len(actAsNotice) > 0 {
		actAsText = actAsNotice[0]
	}
	if actAsText != "" {
		components = append(components, dc.TextDisplay{
			Content: "-# " + actAsText,
		})
	}

	_ = e.Followup(dc.Message{
		Components: components,
		Files:      files,
	})
}

// getColleggtiblePredictions returns the next predicted drop times for colleggtible eggs.
func getColleggtiblePredictions() map[string]time.Time {
	_, wedTime, friTime, ok := contractTimes9amPacific(0)
	if !ok {
		return nil
	}
	preds := predictCollectibles(wedTime, friTime)
	out := make(map[string]time.Time, len(preds)*3)
	for k, v := range preds {
		out[normalizeColleggtibleName(k)] = v.predictedTime
		out[strings.ToLower(k)] = v.predictedTime
		out[k] = v.predictedTime
	}
	return out
}

// BuildColleggtiblesComponents constructs Discord Components V2 layout for colleggtibles.
func BuildColleggtiblesComponents(backup *ei.Backup, targetUserID string, avatarURL string, showAllPredictions bool, customPredictions ...map[string]time.Time) ([]dc.LayoutComponent, []dc.File) {
	storedCollegg := farmerstate.GetMiscSettingString(targetUserID, "collegg")
	var contracts *ei.MyContracts
	if backup != nil {
		contracts = backup.GetContracts()
	}
	overview := ei.GetColleggtiblesOverview(contracts, storedCollegg)

	var preds map[string]time.Time
	if len(customPredictions) > 0 {
		preds = customPredictions[0]
	} else {
		preds = getColleggtiblePredictions()
	}

	userName := ""
	if backup != nil {
		userName = ei.NormalizePlayerNameForDisplay(backup.GetUserName())
	}
	if userName == "" {
		ign := strings.TrimSpace(farmerstate.GetMiscSettingString(targetUserID, "ei_ign"))
		if ign != "" {
			userName = ei.NormalizePlayerNameForDisplay(ign)
		} else {
			userName = "Farmer"
		}
	}

	var header strings.Builder
	fmt.Fprintf(&header, "## %s's Colleggtibles\n", userName)

	if overview.MaxedCount == overview.TotalCount && overview.TotalCount > 0 {
		fmt.Fprintf(&header, "⭐ **%d/%d Maxed**", overview.MaxedCount, overview.TotalCount)
	} else if overview.CollectedCount == overview.TotalCount {
		fmt.Fprintf(&header, "🥚 **%d/%d Collected** · **%d/%d Maxed**", overview.CollectedCount, overview.TotalCount, overview.MaxedCount, overview.TotalCount)
	} else {
		missing := overview.TotalCount - overview.CollectedCount
		fmt.Fprintf(&header, "🥚 **%d/%d Collected** · **%d/%d Maxed** · %d Missing", overview.CollectedCount, overview.TotalCount, overview.MaxedCount, overview.TotalCount, missing)
	}

	var containerSub []dc.ContainerSubComponent
	trimmedHeader := strings.TrimSpace(header.String())
	if avatarURL != "" && (strings.HasPrefix(avatarURL, "http://") || strings.HasPrefix(avatarURL, "https://")) {
		containerSub = append(containerSub, dc.Section{
			Components: []dc.TextDisplay{
				{Content: trimmedHeader},
			},
			Accessory: dc.Thumbnail{
				URL: avatarURL,
			},
		})
	} else {
		containerSub = append(containerSub, dc.TextDisplay{
			Content: trimmedHeader,
		})
	}

	var files []dc.File
	imgBytes, imgErr := RenderColleggtiblesImage(overview, userName, preds, showAllPredictions)
	if imgErr != nil {
		log.Printf("Error rendering colleggtibles image: %v", imgErr)
	}

	if imgErr == nil && len(imgBytes) > 0 {
		files = append(files, dc.File{
			Name:        "colleggtibles.png",
			ContentType: "image/png",
			Reader:      bytes.NewReader(imgBytes),
		})
		containerSub = append(containerSub, dc.MediaGallery{
			Items: []dc.MediaItem{{URL: "attachment://colleggtibles.png"}},
		})
	} else {
		// Fallback to text formatting if image generation fails
		isCoopDelivery := func(d ei.GameModifier_GameDimension) bool {
			return d == ei.GameModifier_EGG_LAYING_RATE ||
				d == ei.GameModifier_SHIPPING_CAPACITY ||
				d == ei.GameModifier_HAB_CAPACITY ||
				d == ei.GameModifier_INTERNAL_HATCHERY_RATE
		}

		var deliveryDims []ei.DimensionBuffSummary
		var economyDims []ei.DimensionBuffSummary
		for _, dim := range overview.Dimensions {
			if isCoopDelivery(dim.Dimension) {
				deliveryDims = append(deliveryDims, dim)
			} else {
				economyDims = append(economyDims, dim)
			}
		}

		anyPredicted := false
		formatDimBlock := func(title string, dims []ei.DimensionBuffSummary) string {
			var sb strings.Builder
			sb.WriteString(title)
			sb.WriteString("\n")
			for i, dim := range dims {
				if i > 0 {
					sb.WriteString("\n")
				}
				if dim.IsMaxed {
					fmt.Fprintf(&sb, "**%s** · **%s** *(Maxed)*\n", dim.DimensionName, dim.CurrentString)
				} else {
					fmt.Fprintf(&sb, "**%s** · **%s** *(%s max)*\n", dim.DimensionName, dim.CurrentString, dim.MaxString)
				}

				for _, c := range dim.Colleggtibles {
					emoji := ei.FindEggEmoji(c.EggID)
					var predTime time.Time
					if preds != nil {
						if t, ok := preds[normalizeColleggtibleName(c.EggID)]; ok {
							predTime = t
						} else if t, ok := preds[strings.ToLower(c.EggID)]; ok {
							predTime = t
						} else if t, ok := preds[normalizeColleggtibleName(c.Name)]; ok {
							predTime = t
						}
					}

					if c.IsMaxed {
						predSuffix := ""
						if showAllPredictions && !predTime.IsZero() {
							predSuffix = fmt.Sprintf(" · 🔮 %s", bottools.WrapTimestamp(predTime.Unix(), bottools.TimestampShortDate))
							anyPredicted = true
						}
						fmt.Fprintf(&sb, "- %s **%s** · ✅ Maxed: %s%s\n", emoji, c.Name, c.CurrentValueString, predSuffix)
					} else {
						predSuffix := ""
						if !predTime.IsZero() {
							predSuffix = fmt.Sprintf(" · 🔮 %s", bottools.WrapTimestamp(predTime.Unix(), bottools.TimestampShortDate))
							anyPredicted = true
						}

						if c.Tier >= 0 {
							sizeStr := ei.FormatEIValue(c.FarmSize, map[string]any{"decimals": 1, "trim": true})
							fmt.Fprintf(&sb, "- %s **%s** · 🟡 Tier %d: %s *(%s/10B)*%s\n", emoji, c.Name, c.Tier+1, c.CurrentValueString, sizeStr, predSuffix)
						} else {
							fmt.Fprintf(&sb, "- %s **%s** · ❌ Missing *(%s max)*%s\n", emoji, c.Name, c.MaxValueString, predSuffix)
						}
					}
				}
			}
			return strings.TrimSpace(sb.String())
		}

		if len(deliveryDims) > 0 {
			containerSub = append(containerSub, dc.Separator{
				Divider: true,
				Spacing: dc.SeparatorSpacingSmall,
			})
			containerSub = append(containerSub, dc.TextDisplay{
				Content: formatDimBlock("### 🚚 Coop & Delivery", deliveryDims),
			})
		}

		if len(economyDims) > 0 {
			containerSub = append(containerSub, dc.Separator{
				Divider: true,
				Spacing: dc.SeparatorSpacingSmall,
			})
			containerSub = append(containerSub, dc.TextDisplay{
				Content: formatDimBlock("### 🌾 Farm & Economy", economyDims),
			})
		}

		if anyPredicted {
			containerSub = append(containerSub, dc.TextDisplay{
				Content: "-# 🔮 Next predicted contract drop date",
			})
		}
	}

	components := []dc.LayoutComponent{
		dc.Container{
			AccentColor: 0x0080FF,
			Components:  containerSub,
		},
	}
	return components, files
}
