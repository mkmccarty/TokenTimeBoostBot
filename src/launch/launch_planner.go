package launch

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	// Goal options
	GoalArtifactHunt   = "artifact_hunt"
	GoalFuelEfficiency = "fuel_efficiency"
	GoalAllStars       = "all_stars"
	GoalEnlightenment  = "enlightenment"
)

func init() {
	boost.RegisterEggIDModalAction("launch_planner", func(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
		HandleLaunchPlannerModal(e, options, encryptedID, okayToSave)
	})
}

// GetSlashLaunchPlannerCommand returns the command definition for /launch-planner.
func GetSlashLaunchPlannerCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Interactive launch planner, ship quality tracker, and fuel advisor.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.StringOption{
				Name:         "alt",
				Description:  "Select an alternate registered account to inspect.",
				Required:     false,
				Autocomplete: true,
			},
			dc.StringOption{
				Name:        "goal",
				Description: "Planning goal: Artifact Hunting, Fuel Efficiency, All Stars, Enlightenment. [Sticky]",
				Required:    false,
				Choices: []dc.Choice[string]{
					{Name: "🏆 Artifact Hunting (Best Ships)", Value: GoalArtifactHunt},
					{Name: "📦 Most Artifacts / Used Fuel (ExHen)", Value: GoalFuelEfficiency},
					{Name: "⭐ All Stars Club (Fastest LP)", Value: GoalAllStars},
					{Name: "🧘 Maximize Enlightenment Run", Value: GoalEnlightenment},
				},
			},
			dc.BoolOption{
				Name:        "dubcap",
				Description: "Consider next Sunday 48h Double Capacity event in timing. [Sticky]",
				Required:    false,
			},
			dc.IntOption{
				Name:        "target-launches",
				Description: "Target number of launches for fuel planning (e.g. 3, 6, 9, 12, 15, 21, 30). [Sticky]",
				Required:    false,
			},
			dc.BoolOption{
				Name:        "help",
				Description: "Show explanation and help for the launch planner.",
				Required:    false,
			},
			dc.BoolOption{
				Name:        "reset",
				Description: "Clear saved Egg Inc ID registration for this command.",
				Required:    false,
			},
		},
	}
	return &command
}

// HandleLaunchPlannerAutocomplete handles autocomplete events for the /launch-planner command.
func HandleLaunchPlannerAutocomplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	if name != "alt" {
		return
	}

	alts := farmerstate.GetUserAltsWithSavedEID(farmerstate.GetEffectiveUserID(e.UserID(), e.ChannelID()))
	value = strings.ToLower(strings.TrimSpace(value))

	var choices []dc.Choice[string]
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

func getEggIncID(userID string) string {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	return decryptEggIncID(eiID)
}

// HandleLaunchPlannerCommand handles the /launch-planner command invocation.
func HandleLaunchPlannerCommand(e *dc.CommandEvent) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	if opt, ok := e.OptBool("help"); ok && opt {
		_ = e.Respond(dc.Message{Content: launchPlannerText(), Ephemeral: true})
		return
	}

	if opt, ok := e.OptBool("reset"); ok && opt {
		farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
	}

	altParam, _ := e.OptString("alt")
	targetID, altEggIncID, notice := farmerstate.ResolveAltSelection(userID, altParam, "Launch Planner")

	if isActAs && altParam == "" {
		var actAsNotice string
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = "Operating as linked account."
		} else {
			remaining := time.Until(expiresAt).Round(time.Second)
			actAsNotice = fmt.Sprintf("Operating as linked account for %s.", remaining)
		}
		if notice == "" {
			notice = actAsNotice
		} else {
			notice = actAsNotice + " " + notice
		}
	}

	// Update sticky settings if options were passed in the slash command
	if opt, ok := e.OptString("goal"); ok {
		farmerstate.SetMiscSettingString(targetID, "launch_planner_goal", opt)
	}
	if opt, ok := e.OptBool("dubcap"); ok {
		farmerstate.SetMiscSettingFlag(targetID, "launch_planner_dubcap", opt)
	}
	if opt, ok := e.OptInt("target-launches"); ok && opt > 0 {
		farmerstate.SetMiscSettingString(targetID, "launch_planner_target", strconv.Itoa(opt))
	}

	eggIncID := altEggIncID
	if eggIncID == "" {
		eggIncID = getEggIncID(targetID)
	}

	if eggIncID == "" {
		boost.RequestEggIncIDModal(e, "launch_planner", e.Options())
		return
	}

	_ = e.Defer(true)

	backup, _ := ei.GetFirstContactFromAPI(eggIncID, targetID, true)
	if backup == nil {
		_ = e.Followup(dc.Message{
			Content:   "Unable to retrieve game data for your Egg Inc ID. Please verify your ID and try again.",
			Ephemeral: true,
		})
		return
	}

	farmerstate.SetFarmerBackupDetails(targetID, backup)

	msg := buildPlannerDialog(targetID, backup, notice)
	_ = e.Followup(msg)
}

// HandleLaunchPlannerModal handles the modal submission when user supplies their Egg Inc ID.
func HandleLaunchPlannerModal(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
	effectiveUserID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(e.UserID(), e.ChannelID())
	userID := effectiveUserID

	altParam, _ := options.String("alt")
	targetID, altEggIncID, notice := farmerstate.ResolveAltSelection(userID, altParam, "Launch Planner")

	if isActAs && altParam == "" {
		var actAsNotice string
		if farmerstate.IsActAsForever(expiresAt) {
			actAsNotice = "Operating as linked account."
		} else {
			remaining := time.Until(expiresAt).Round(time.Second)
			actAsNotice = fmt.Sprintf("Operating as linked account for %s.", remaining)
		}
		if notice == "" {
			notice = actAsNotice
		} else {
			notice = actAsNotice + " " + notice
		}
	}

	eggIncID := altEggIncID
	if eggIncID == "" {
		eggIncID = decryptEggIncID(encryptedID)
	}

	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		_ = e.Respond(dc.Message{
			Content:   "Invalid Egg Inc ID provided. Please enter a valid ID in the format EI1234567890123456.",
			Ephemeral: true,
		})
		return
	}

	if okayToSave {
		farmerstate.SetMiscSettingString(targetID, "encrypted_ei_id", encryptedID)
	}

	_ = e.Defer(true)

	backup, _ := ei.GetFirstContactFromAPI(eggIncID, targetID, okayToSave)
	if backup == nil {
		_ = e.Followup(dc.Message{
			Content:   "Unable to retrieve game data for your Egg Inc ID. Please verify your ID and try again.",
			Ephemeral: true,
		})
		return
	}

	farmerstate.SetFarmerBackupDetails(targetID, backup)
	msg := buildPlannerDialog(targetID, backup, notice)
	_ = e.Followup(msg)
}

// HandleLaunchPlannerComponent handles interactive select menu and button clicks.
func HandleLaunchPlannerComponent(e *dc.ComponentEvent) {
	customID := e.CustomID()
	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		return
	}

	action := parts[1]
	targetID := e.UserID()
	if len(parts) >= 3 && parts[2] != "" {
		targetID = parts[2]
	}

	// Apply interactions
	switch action {
	case "goal":
		if len(e.Values()) > 0 {
			farmerstate.SetMiscSettingString(targetID, "launch_planner_goal", e.Values()[0])
		}
	case "dubcap":
		if len(e.Values()) > 0 {
			val := e.Values()[0] == "true"
			farmerstate.SetMiscSettingFlag(targetID, "launch_planner_dubcap", val)
		}
	case "target":
		if len(e.Values()) > 0 {
			farmerstate.SetMiscSettingString(targetID, "launch_planner_target", e.Values()[0])
		}
	case "refresh":
		// Just re-queries and updates
	case "info":
		_ = e.Respond(dc.Message{
			Content:   launchPlannerText(),
			Ephemeral: true,
		})
		return
	}

	eggIncID := getEggIncID(targetID)
	if eggIncID == "" {
		_ = e.Respond(dc.Message{
			Content:   "Egg Inc ID not found. Please run `/launch-planner` to register your ID.",
			Ephemeral: true,
		})
		return
	}

	backup, _ := ei.GetFirstContactFromAPI(eggIncID, targetID, true)
	if backup == nil {
		_ = e.Respond(dc.Message{
			Content:   "Unable to refresh game data. Please try again later.",
			Ephemeral: true,
		})
		return
	}

	msg := buildPlannerDialog(targetID, backup, "")
	_ = e.Update(msg)
}

func launchPlannerText() string {
	var b strings.Builder
	b.WriteString("# 🚀 /launch-planner - Interactive Launch Planner\n\n")
	b.WriteString("The `/launch-planner` tool automatically inspects your farm data to plan rocket launches and fuel requirements:\n\n")
	b.WriteString("- **Home Farm vs Path of Virtue (PoV)**: Automatically detects whether your current home farm is producing regular eggs or Eggs of Virtue.\n")
	b.WriteString("- **Returning Rockets & Quality**: Shows active missions, exact return times `<t:time:R>`, ship stars, quality rating, and capacity.\n")
	b.WriteString("- **Fuel Advisor & Bottle-neck Warning**: Tracks fuel tank inventory against what you've been launching and warns you before tanks run dry so you know when to refill.\n")
	b.WriteString("- **Virtue Farm Mode**: Launches only occur on the **Egg of Humility**. If you are on another virtue egg, it calculates how much of the current egg is needed to hit your launch target, ignoring Humility egg fuel.\n")
	b.WriteString("- **Goals**:\n")
	b.WriteString("  - 🏆 **Artifact Hunting**: Focuses on maximum quality ships (Extended Henliner or Henerprise).\n")
	b.WriteString("  - 📦 **Most Artifacts / Used Fuel**: Focuses on Extended Henerprise (ExHen), saving ~2.35x fuel on Virtue farms to maximize total artifact drops per tank.\n")
	b.WriteString("  - ⭐ **All Stars Club**: Focuses on Short missions for ships needing stars to achieve ASC in the fastest time.\n")
	b.WriteString("  - 🧘 **Enlightenment Run**: Calculates optimal tank storage and launch capacity to sustain launches throughout an Enlightenment run without refueling.\n")
	b.WriteString("- **Sunday Double Capacity Event (48h)**: Analyzes next Sunday's event window (9 AM PT Sunday to Tuesday) and provides timing advice so you never miss a 2x capacity launch!\n")
	return b.String()
}

func formatEggName(egg ei.Egg) string {
	name, ok := ei.Egg_name[int32(egg)]
	if !ok {
		return "Unknown"
	}
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}

// isVirtueMission returns true if the mission was launched as a Virtue mission.
func isVirtueMission(m *ei.MissionInfo) bool {
	if m == nil {
		return false
	}
	if m.GetType() == ei.MissionInfo_VIRTUE {
		return true
	}
	for _, f := range m.GetFuel() {
		if f != nil {
			egg := f.GetEgg()
			if egg >= ei.Egg_CURIOSITY && egg <= ei.Egg_KINDNESS {
				return true
			}
		}
	}
	return false
}

// isExactOrBorderlineFuel checks if stored fuel is exactly at or barely above the needed amount
// with no safe buffer against Egg, Inc.'s floating-point rounding bugs.
func isExactOrBorderlineFuel(stored, needed float64) bool {
	if needed <= 0 || stored < needed {
		return false
	}
	diff := stored - needed
	// If exactly equal, always true
	if diff == 0 || diff < 1.0 {
		return true
	}
	// Buffer threshold: up to 100B, but at most 0.2% of needed
	threshold := math.Min(100e9, needed*0.002)
	if threshold < 1.0 {
		threshold = 1.0
	}
	return diff <= threshold
}

// buildPlannerDialog builds the Discord message with rich layout components.
func buildPlannerDialog(targetID string, backup *ei.Backup, notice string) dc.Message {
	isVirtueFarm := false
	var farmEgg = ei.Egg_UNKNOWN
	var homeFarm *ei.Backup_Simulation
	if len(backup.GetFarms()) > 0 && backup.GetFarms()[0] != nil {
		homeFarm = backup.GetFarms()[0]
	} else if backup.GetSim() != nil {
		homeFarm = backup.GetSim()
	}
	if homeFarm != nil {
		farmEgg = homeFarm.GetEggType()
		if farmEgg >= ei.Egg_CURIOSITY && farmEgg <= ei.Egg_KINDNESS {
			isVirtueFarm = true
		}
	}

	// Read sticky settings
	goal := farmerstate.GetMiscSettingString(targetID, "launch_planner_goal")
	if goal == "" {
		if isVirtueFarm {
			goal = GoalFuelEfficiency
		} else {
			goal = GoalArtifactHunt
		}
	} else if isVirtueFarm && goal == GoalEnlightenment {
		goal = GoalFuelEfficiency
	}
	considerDubcap := farmerstate.GetMiscSettingFlag(targetID, "launch_planner_dubcap")

	targetLaunchesStr := farmerstate.GetMiscSettingString(targetID, "launch_planner_target")
	targetLaunches := 9 // Default 9 launches for normal farm (3 full tri-launch cycles)
	if isVirtueFarm {
		targetLaunches = 3 // Default 3 launches for virtue farm (1 full tri-launch cycle fits in tank)
	}
	if targetLaunchesStr != "" {
		if val, err := strconv.Atoi(targetLaunchesStr); err == nil && val > 0 {
			targetLaunches = val
		}
	}

	tankLevels := []float64{2e9, 200e9, 10e12, 100e12, 200e12, 300e12, 400e12, 500e12}

	var components []dc.LayoutComponent
	if notice != "" {
		components = append(components, dc.TextDisplay{Content: notice})
	}

	var header strings.Builder
	displayName := ei.NormalizePlayerNameForDisplay(backup.GetUserName())
	farmName := formatEggName(farmEgg)
	farmEmoji, _ := ei.GetEggEmojiMarkdownIfExists(farmEgg)

	fmt.Fprintf(&header, "# 🚀 Launch Planner & Advisor\n**Farmer**: **%s**\n", displayName)
	if isVirtueFarm {
		fmt.Fprintf(&header, "**Farm**: %s Path of Virtue (%s)\n", farmEmoji, farmName)
	} else {
		fmt.Fprintf(&header, "**Farm**: %s %s Farm\n", farmEmoji, farmName)
	}

	// Epic Research multiplier for capacity
	epicCapacityMult := 1.0
	if backup.GetGame() != nil {
		epicCapacityMult = ei.GetEpicResearchMissionCapacity(backup.GetGame().GetEpicResearch())
	}

	// 1. ACTIVE MISSIONS SECTION
	var activeSec strings.Builder
	if isVirtueFarm {
		activeSec.WriteString("### 🛸 Returning Virtue Rockets & Ship Quality\n")
	} else {
		activeSec.WriteString("### 🛸 Returning Rockets & Ship Quality\n")
	}

	afxDB := backup.GetArtifactsDb()
	var activeMissions []*ei.MissionInfo
	if afxDB != nil {
		for _, m := range afxDB.GetMissionInfos() {
			if m != nil && m.GetStatus() >= ei.MissionInfo_FUELING && m.GetStatus() <= ei.MissionInfo_ANALYZING {
				if isVirtueFarm != isVirtueMission(m) {
					continue
				}
				activeMissions = append(activeMissions, m)
			}
		}
	}

	if len(activeMissions) == 0 {
		if isVirtueFarm {
			activeSec.WriteString("No active Virtue rockets currently deployed. Ready to launch!\n")
		} else {
			activeSec.WriteString("No active rockets currently deployed. Ready to launch!\n")
		}
	} else {
		for _, m := range activeMissions {
			shipType := m.GetShip()
			durType := m.GetDurationType()
			shipName := ei.MissionInfo_Spaceship_name[int32(shipType)]
			if int(shipType) < len(ei.MissionArt.Ships) {
				shipName = ei.MissionArt.Ships[shipType].Name
			}
			shipArt := ""
			if int(shipType) < len(ei.MissionArt.Ships) {
				shipArt = ei.GetBotEmojiMarkdown(ei.MissionArt.Ships[shipType].Art) + " "
			}

			durName := "Short"
			switch durType {
			case ei.MissionInfo_LONG:
				durName = "Standard"
			case ei.MissionInfo_EPIC:
				durName = "Extended"
			case ei.MissionInfo_TUTORIAL:
				durName = "Tutorial"
			}

			level := m.GetLevel()
			starsStr := fmt.Sprintf("⭐%d", level)

			targetStr := ""
			if target, ok := getTargetArtifact(m); ok {
				if emoji := getTargetArtifactEmoji(target); emoji != "" {
					targetStr = fmt.Sprintf(" • 🎯 %s", emoji)
				} else {
					targetStr = fmt.Sprintf(" • 🎯 %s", getTargetArtifactName(target))
				}
			}

			// Quality & Capacity calculation
			durParam, _ := ei.GetShipMissionParams(shipType, durType)
			calcQuality := durParam.Quality + durParam.LevelQualityBump*float64(level)
			calcMaxQuality := durParam.MaxQuality + durParam.LevelQualityBump*float64(level)
			calcCapacity := uint32(math.Floor(float64(durParam.Capacity+durParam.LevelCapacityBump*level) * epicCapacityMult))
			if m.GetCapacity() > 0 {
				calcCapacity = m.GetCapacity()
			}

			returnSec := int64(m.GetStartTimeDerived() + m.GetDurationSeconds())
			if returnSec == 0 && m.GetSecondsRemaining() > 0 {
				returnSec = time.Now().Unix() + int64(m.GetSecondsRemaining())
			}

			timeStr := ""
			if returnSec > 0 {
				if time.Now().Unix() >= returnSec {
					timeStr = "🟢 **Ready to Collect!**"
				} else {
					timeStr = fmt.Sprintf("Returns <t:%d:R> (<t:%d:t>)", returnSec, returnSec)
				}
			} else {
				timeStr = "In Progress"
			}

			fmt.Fprintf(&activeSec, "> %s**%s** (%s, %s)%s\n", shipArt, shipName, durName, starsStr, targetStr)
			fmt.Fprintf(&activeSec, "> -# ⏱️ %s | ✨ Quality: **%.2f** (Max: %.2f) | 📦 Cap: **%d**\n",
				timeStr, calcQuality, calcMaxQuality, calcCapacity)
		}
	}

	// 2. FUEL & GOAL PLANNING SECTION
	var fuelSec strings.Builder

	if isVirtueFarm {
		renderVirtueFuelPlanning(&fuelSec, backup, farmEgg, goal, tankLevels)
	} else {
		renderNormalFuelPlanning(&fuelSec, backup, farmEgg, goal, targetLaunches, tankLevels)
	}

	// 3. SUNDAY DOUBLE CAPACITY SECTION
	var dubcapSec strings.Builder
	if considerDubcap {
		renderSundayDubcapAdvisor(&dubcapSec, activeMissions, backup)
	}

	// Assemble Container Components
	var containerSubs []dc.ContainerSubComponent

	containerSubs = append(containerSubs, dc.TextDisplay{Content: header.String()})
	containerSubs = append(containerSubs, dc.Separator{Divider: true, Spacing: dc.SeparatorSpacingSmall})

	containerSubs = append(containerSubs, dc.TextDisplay{Content: activeSec.String()})
	containerSubs = append(containerSubs, dc.Separator{Divider: true, Spacing: dc.SeparatorSpacingSmall})

	containerSubs = append(containerSubs, dc.TextDisplay{Content: fuelSec.String()})

	if considerDubcap && dubcapSec.Len() > 0 {
		containerSubs = append(containerSubs, dc.Separator{Divider: true, Spacing: dc.SeparatorSpacingSmall})
		containerSubs = append(containerSubs, dc.TextDisplay{Content: dubcapSec.String()})
	}

	// Dropdown 1: Goal SelectMenu
	var goalOptions []dc.SelectOption
	if isVirtueFarm {
		goalOptions = []dc.SelectOption{
			{
				Label:       "Most Artifacts / Used Fuel",
				Value:       GoalFuelEfficiency,
				Description: "Launch Extended Henerprise (ExHen) for maximum artifacts per fuel",
				Default:     goal == GoalFuelEfficiency,
			},
			{
				Label:       "Artifact Hunting (Henliner)",
				Value:       GoalArtifactHunt,
				Description: "Launch Extended Henliner for highest-tier loot",
				Default:     goal == GoalArtifactHunt,
			},
			{
				Label:       "All Stars Club",
				Value:       GoalAllStars,
				Description: "Launch Short missions to max stars and launch points fastest",
				Default:     goal == GoalAllStars,
			},
		}
	} else {
		goalOptions = []dc.SelectOption{
			{
				Label:       "Artifact Hunting",
				Value:       GoalArtifactHunt,
				Description: "Launch highest-tier ships (Extended Henliner) for best loot",
				Default:     goal == GoalArtifactHunt,
			},
			{
				Label:       "Most Artifacts / Used Fuel",
				Value:       GoalFuelEfficiency,
				Description: "Launch Extended Henerprise to save fuel and maximize artifacts",
				Default:     goal == GoalFuelEfficiency,
			},
			{
				Label:       "All Stars Club",
				Value:       GoalAllStars,
				Description: "Launch Short missions to max stars and launch points fastest",
				Default:     goal == GoalAllStars,
			},
			{
				Label:       "Enlightenment Run",
				Value:       GoalEnlightenment,
				Description: "Plan tank fills to maximize launches without refueling",
				Default:     goal == GoalEnlightenment,
			},
		}
	}

	goalMenu := dc.SelectMenu{
		CustomID:    fmt.Sprintf("launch_planner#goal#%s", targetID),
		Placeholder: "Select Planning Goal...",
		Options:     goalOptions,
	}

	// Dropdown 2: Double Capacity SelectMenu
	dubcapMenu := dc.SelectMenu{
		CustomID:    fmt.Sprintf("launch_planner#dubcap#%s", targetID),
		Placeholder: "Consider Sunday Double Capacity Event?",
		Options: []dc.SelectOption{
			{
				Label:       "Sunday Dubcap: Consider (48h)",
				Value:       "true",
				Description: "Align missions to take advantage of 2x artifact capacity",
				Default:     considerDubcap,
			},
			{
				Label:       "Sunday Dubcap: Standard Schedule",
				Value:       "false",
				Description: "Regular continuous launches ignoring event timing",
				Default:     !considerDubcap,
			},
		},
	}

	// Action buttons
	refreshBtn := dc.Button{
		Label:    "Refresh Data",
		CustomID: fmt.Sprintf("launch_planner#refresh#%s", targetID),
		Style:    dc.ButtonSecondary,
	}
	infoBtn := dc.Button{
		Label:    "Help / Info",
		CustomID: fmt.Sprintf("launch_planner#info#%s", targetID),
		Style:    dc.ButtonSecondary,
	}

	containerSubs = append(containerSubs, dc.Separator{Divider: true, Spacing: dc.SeparatorSpacingSmall})
	containerSubs = append(containerSubs, dc.ActionRow{Components: []dc.InteractiveComponent{goalMenu}})
	containerSubs = append(containerSubs, dc.ActionRow{Components: []dc.InteractiveComponent{dubcapMenu}})
	containerSubs = append(containerSubs, dc.ActionRow{Components: []dc.InteractiveComponent{refreshBtn, infoBtn}})

	accentColor := 0x2ECC71 // Emerald Green
	if isVirtueFarm {
		accentColor = 0x9B59B6 // Purple for Virtue
	}

	components = append(components, dc.Container{
		AccentColor: accentColor,
		Components:  containerSubs,
	})

	return dc.Message{
		Components: components,
		Ephemeral:  true,
	}
}

// renderVirtueFuelPlanning renders the fuel advisor for Virtue farms.
func renderVirtueFuelPlanning(b *strings.Builder, backup *ei.Backup, farmEgg ei.Egg, goal string, tankLevels []float64) {
	b.WriteString("### 🧪 Path of Virtue Fuel Planning\n")

	virtue := backup.GetVirtue()
	if virtue == nil || virtue.GetAfx() == nil {
		b.WriteString("No Virtue AFX data found in player backup.\n")
		return
	}

	afx := virtue.GetAfx()
	tankLevel := 0
	if backup.GetArtifacts() != nil {
		tankLevel = int(backup.GetArtifacts().GetTankLevel())
	}
	if tankLevel >= len(tankLevels) {
		tankLevel = len(tankLevels) - 1
	}
	tankCapacity := tankLevels[tankLevel]

	fuels := afx.GetTankFuels()
	if len(fuels) > 5 {
		fuels = fuels[len(fuels)-5:]
	}
	// Index 0: Curiosity, 1: Integrity, 2: Humility, 3: Resilience, 4: Kindness
	virtueEggs := []ei.Egg{ei.Egg_CURIOSITY, ei.Egg_INTEGRITY, ei.Egg_HUMILITY, ei.Egg_RESILIENCE, ei.Egg_KINDNESS}

	totalVirtueFuel := 0.0
	for _, f := range fuels {
		totalVirtueFuel += f
	}

	tankUsagePct := (totalVirtueFuel / tankCapacity) * 100

	freeTankSpace := math.Max(0, tankCapacity-totalVirtueFuel)
	depotArt := ei.GetBotEmojiMarkdown("depot")
	fmt.Fprintf(b, "%s **Virtue Tank**: %s / %s (%.1f%% full • %s free)\n",
		depotArt,
		ei.FormatEIValue(totalVirtueFuel, map[string]any{"decimals": 2, "trim": true}),
		ei.FormatEIValue(tankCapacity, map[string]any{"decimals": 0, "trim": true}),
		tankUsagePct,
		ei.FormatEIValue(freeTankSpace, map[string]any{"decimals": 2, "trim": true}))

	// Farm state check
	isOnHumility := farmEgg == ei.Egg_HUMILITY
	if isOnHumility {
		b.WriteString("✅ Currently on **Egg of Humility**. Rocket launches are **ACTIVE**!\n")
		b.WriteString("-# Humility fuel is provided continuously from this farm. Other fuels are drawn from the tank.\n\n")
	} else {
		eggName := formatEggName(farmEgg)
		fmt.Fprintf(b, "⚠️ Currently on **Egg of %s**. Launches **CANNOT** occur on this egg!\n", eggName)
		b.WriteString("-# ℹ️ Launches only happen on **Humility**. Plan which eggs to visit before heading to Humility.\n\n")
	}

	// Evaluate Target Ship
	targetShip := ei.MissionInfo_ATREGGIES
	targetDur := ei.MissionInfo_EPIC
	shipName := "Extended Atreggies Henliner"

	switch goal {
	case GoalFuelEfficiency:
		targetShip = ei.MissionInfo_HENERPRISE
		targetDur = ei.MissionInfo_EPIC
		shipName = "Extended Henerprise"
	case GoalAllStars:
		shipName, targetShip, targetDur = findNextAllStarsShip(backup, true)
	}

	reqs := ei.GetMissionFuels(targetShip, targetDur, true)

	// Calculate total non-Humility tank fuel needed across all egg types
	totalTankFuelPerLaunch := 0.0
	for _, req := range reqs {
		if req.Egg != ei.Egg_HUMILITY {
			totalTankFuelPerLaunch += req.Amount
		}
	}

	maxLaunchesTankCanHold := 0
	if totalTankFuelPerLaunch > 0 {
		maxLaunchesTankCanHold = int(math.Floor(tankCapacity / totalTankFuelPerLaunch))
	}

	// In Virtue farm, the number of launches is calculated directly from tank capacity:
	targetLaunches := maxLaunchesTankCanHold
	if targetLaunches <= 0 {
		targetLaunches = 1
	}

	totalTankFuelNeeded := totalTankFuelPerLaunch * float64(targetLaunches)

	fmt.Fprintf(b, "🎯 **Goal**: %s (`%s`)\n", getGoalLabel(goal), shipName)
	if goal == GoalFuelEfficiency {
		b.WriteString("-# 💡 *ExHen requires ~2.35x less tank fuel than Henliner (70T vs 165T), maximizing artifact drops per tank fill.*\n")
	}

	if maxLaunchesTankCanHold > 0 {
		fmt.Fprintf(b, "📊 **Calculated Launch Capacity**: **%d launches** (Full tank holds %s for %s)\n\n",
			targetLaunches,
			ei.FormatEIValue(totalTankFuelNeeded, map[string]any{"decimals": 1, "trim": true}),
			shipName)
	} else {
		fmt.Fprintf(b, "⚠️ **Tank Upgrade Needed**: Tank holds %s, but 1 launch requires %s across all eggs.\n\n",
			ei.FormatEIValue(tankCapacity, map[string]any{"decimals": 0, "trim": true}),
			ei.FormatEIValue(totalTankFuelPerLaunch, map[string]any{"decimals": 1, "trim": true}))
	}

	b.WriteString("**Required Fuel Breakdown** (Humility ignored):\n")

	type eggPlan struct {
		egg           ei.Egg
		stored        float64
		needed        float64
		deficit       float64
		launchesCanDo float64
		isCurrent     bool
	}

	var eggsNeedingVisit []string
	var roundingRiskEggs []string
	currentEggIsRequired := false
	var currentEggPlan eggPlan
	minPossibleLaunches := math.MaxFloat64
	var bottleneckEgg ei.Egg

	for _, req := range reqs {
		if req.Egg == ei.Egg_HUMILITY {
			continue // Ignored as per spec
		}

		eggIdx := int(req.Egg - ei.Egg_CURIOSITY)
		currentStored := 0.0
		if eggIdx >= 0 && eggIdx < len(fuels) {
			currentStored = fuels[eggIdx]
		}

		totalNeeded := req.Amount * float64(targetLaunches)
		launchesCanDo := currentStored / req.Amount
		if launchesCanDo < minPossibleLaunches {
			minPossibleLaunches = launchesCanDo
			bottleneckEgg = req.Egg
		}

		deficit := totalNeeded - currentStored
		isCurrent := req.Egg == farmEgg

		p := eggPlan{
			egg:           req.Egg,
			stored:        currentStored,
			needed:        totalNeeded,
			deficit:       deficit,
			launchesCanDo: launchesCanDo,
			isCurrent:     isCurrent,
		}

		if isCurrent {
			currentEggIsRequired = true
			currentEggPlan = p
		}

		if deficit > 0 {
			eggsNeedingVisit = append(eggsNeedingVisit, formatEggName(req.Egg))
		}

		eggEmoji, _ := ei.GetEggEmojiMarkdownIfExists(req.Egg)
		eggName := formatEggName(req.Egg)

		currIndicator := ""
		if isCurrent {
			currIndicator = " 👈 *(Current Farm)*"
		}

		isExact := isExactOrBorderlineFuel(currentStored, totalNeeded)
		if isExact {
			roundingRiskEggs = append(roundingRiskEggs, formatEggName(req.Egg))
		}

		statusStr := ""
		if deficit > 0 {
			statusStr = fmt.Sprintf("• Needs **+%s**", ei.FormatEIValue(deficit, map[string]any{"decimals": 1, "trim": true}))
		} else if isExact {
			statusStr = "• ⚠️ **Exact! (Rounding Risk)**"
		} else {
			statusStr = "• ✅ **Sufficient**"
		}

		fmt.Fprintf(b, "- %s **%s**: %s / %s (has fuel for %.1f launches) %s%s\n",
			eggEmoji, eggName,
			ei.FormatEIValue(currentStored, map[string]any{"decimals": 1, "trim": true}),
			ei.FormatEIValue(totalNeeded, map[string]any{"decimals": 1, "trim": true}),
			launchesCanDo,
			statusStr,
			currIndicator)
	}

	b.WriteString("\n")

	if len(roundingRiskEggs) > 0 {
		fmt.Fprintf(b, "⚠️ **Fuel Tank Rounding Warning**: **%s** is exactly at the needed fuel amount with no buffer. Egg, Inc. often suffers from floating-point rounding errors on exact tank amounts, which can prevent the final rocket launch. It is strongly recommended to bank a small extra buffer (+10B–100B) to guarantee successful launches!\n\n",
			strings.Join(roundingRiskEggs, ", "))
	}

	// Check for useless / wasted fuels in tank
	requiredEggAmounts := make(map[ei.Egg]float64)
	for _, req := range reqs {
		if req.Egg != ei.Egg_HUMILITY {
			requiredEggAmounts[req.Egg] = req.Amount * float64(targetLaunches)
		}
	}

	type uselessFuelInfo struct {
		name   string
		amount float64
		reason string
	}
	var uselessFuels []uselessFuelInfo
	totalUselessAmount := 0.0

	for i, egg := range virtueEggs {
		if i >= len(fuels) {
			continue
		}
		stored := fuels[i]
		if stored < 1e9 { // Less than 1B is negligible
			continue
		}

		if egg == ei.Egg_HUMILITY {
			uselessFuels = append(uselessFuels, uselessFuelInfo{
				name:   "Humility",
				amount: stored,
				reason: "Humility is pumped directly on-farm during launches; tank storage is wasted space",
			})
			totalUselessAmount += stored
		} else if needed, isReq := requiredEggAmounts[egg]; !isReq {
			uselessFuels = append(uselessFuels, uselessFuelInfo{
				name:   formatEggName(egg),
				amount: stored,
				reason: fmt.Sprintf("Not used by %s", shipName),
			})
			totalUselessAmount += stored
		} else if stored > needed+1e11 { // More than 100B over needed target
			excess := stored - needed
			uselessFuels = append(uselessFuels, uselessFuelInfo{
				name:   fmt.Sprintf("%s (Excess)", formatEggName(egg)),
				amount: excess,
				reason: fmt.Sprintf("Exceeds the %s needed for %d launches", ei.FormatEIValue(needed, map[string]any{"decimals": 1, "trim": true}), targetLaunches),
			})
			totalUselessAmount += excess
		}
	}

	if len(uselessFuels) > 0 {
		b.WriteString("🗑️ **Tank Waste & Cleanup Recommendations**:\n")
		for _, u := range uselessFuels {
			fmt.Fprintf(b, "• ❌ **%s**: **%s** stored • %s.\n",
				u.name,
				ei.FormatEIValue(u.amount, map[string]any{"decimals": 2, "trim": true}),
				u.reason)
		}
		fmt.Fprintf(b, "-# 💡 Clearing unused or excess fuels in-game frees up **%s** of tank space to make room for required fuels!\n\n",
			ei.FormatEIValue(totalUselessAmount, map[string]any{"decimals": 2, "trim": true}))
	}

	// Fuel advice and plan
	if !isOnHumility {
		b.WriteString("🗺️ **Path of Virtue Fueling Plan**:\n")
		if len(eggsNeedingVisit) > 0 {
			fmt.Fprintf(b, "• 🎯 **Eggs to Visit**: **%s** must be visited to gather required fuel before heading to Humility.\n",
				strings.Join(eggsNeedingVisit, ", "))
		} else {
			fmt.Fprintf(b, "• 🎉 **All Required Tank Fuels Banked!** You already have enough stored fuel for %d launches.\n", targetLaunches)
		}

		// Current farm action
		if currentEggIsRequired {
			if currentEggPlan.deficit > 0 {
				fillTarget := currentEggPlan.deficit
				if fillTarget > freeTankSpace {
					fmt.Fprintf(b, "• ⛽ **Current Farm (%s)**: Needs **+%s**, but only **%s** free space remains in tank across all eggs. Bank as much as tank allows before shifting!\n",
						formatEggName(farmEgg),
						ei.FormatEIValue(fillTarget, map[string]any{"decimals": 1, "trim": true}),
						ei.FormatEIValue(freeTankSpace, map[string]any{"decimals": 1, "trim": true}))
				} else {
					fmt.Fprintf(b, "• ⛽ **Current Farm (%s)**: Bank **+%s** more fuel here into your tank before shifting forward.\n",
						formatEggName(farmEgg),
						ei.FormatEIValue(fillTarget, map[string]any{"decimals": 1, "trim": true}))
				}
			} else if isExactOrBorderlineFuel(currentEggPlan.stored, currentEggPlan.needed) {
				fmt.Fprintf(b, "• ⚠️ **Current Farm (%s)**: Stored fuel is exactly at the required amount (%s for %d launches). Due to Egg, Inc. fuel tank rounding bugs, bank a small extra buffer (+10B–100B) before shifting so your final launch isn't rejected!\n",
					formatEggName(farmEgg),
					ei.FormatEIValue(currentEggPlan.stored, map[string]any{"decimals": 1, "trim": true}),
					targetLaunches)
			} else {
				fmt.Fprintf(b, "• ✅ **Current Farm (%s)**: Target already met (has fuel for %.1f launches)! No more fuel needed here; safe to shift toward other required eggs or Humility.\n",
					formatEggName(farmEgg), currentEggPlan.launchesCanDo)
			}
		} else {
			fmt.Fprintf(b, "• ℹ️ **Current Farm (%s)**: Not required for %s. Safe to shift forward toward the next required egg or Humility.\n",
				formatEggName(farmEgg), shipName)
		}

		if len(eggsNeedingVisit) == 0 {
			b.WriteString("• 🚀 **Next Destination**: Proceed along the path directly to **Humility** to start launches!\n")
		}
	} else {
		// On Humility
		if minPossibleLaunches < float64(targetLaunches) {
			fmt.Fprintf(b, "⚠️ **Fuel Warning**: Tank has non-Humility fuels for only **%d** launches (bottleneck: **%s**). Prepare to plan your next fueling circuit after these launches!\n",
				int(minPossibleLaunches), formatEggName(bottleneckEgg))
		} else {
			fmt.Fprintf(b, "✅ **Fuel Ready**: Tank contains enough fuel for all **%d** target launches!\n", targetLaunches)
		}
	}
	_ = virtueEggs
}

// renderNormalFuelPlanning renders the fuel advisor for Standard home farms.
func renderNormalFuelPlanning(b *strings.Builder, backup *ei.Backup, farmEgg ei.Egg, goal string, targetLaunches int, tankLevels []float64) {
	b.WriteString("### ⛽ Fuel Tank Advisor & Launch Planning\n")

	artifacts := backup.GetArtifacts()
	if artifacts == nil {
		b.WriteString("No Artifacts tank data found in player backup.\n")
		return
	}

	tankLevel := int(artifacts.GetTankLevel())
	if tankLevel >= len(tankLevels) {
		tankLevel = len(tankLevels) - 1
	}
	tankCapacity := tankLevels[tankLevel]

	tankFuels := artifacts.GetTankFuels()
	totalTankFuel := 0.0
	for _, f := range tankFuels {
		totalTankFuel += f
	}

	depotArt := ei.GetBotEmojiMarkdown("depot")
	tankUsagePct := (totalTankFuel / tankCapacity) * 100

	fmt.Fprintf(b, "%s **Fuel Tank**: %s / %s (Level %d, %.1f%% full)\n\n",
		depotArt,
		ei.FormatEIValue(totalTankFuel, map[string]any{"decimals": 2, "trim": true}),
		ei.FormatEIValue(tankCapacity, map[string]any{"decimals": 0, "trim": true}),
		tankLevel,
		tankUsagePct)

	// Determine planned ship
	targetShip := ei.MissionInfo_ATREGGIES
	targetDur := ei.MissionInfo_EPIC
	shipName := "Extended Atreggies Henliner"

	switch goal {
	case GoalFuelEfficiency:
		targetShip = ei.MissionInfo_HENERPRISE
		targetDur = ei.MissionInfo_EPIC
		shipName = "Extended Henerprise"
	case GoalAllStars:
		shipName, targetShip, targetDur = findNextAllStarsShip(backup, false)
	case GoalEnlightenment:
		renderEnlightenmentRunAdvisor(b, backup, tankCapacity)
		return
	default:
		// Artifact hunting: Use Henliner if available, otherwise Henerprise
		if !hasUnlockedHenliner(backup) {
			targetShip = ei.MissionInfo_HENERPRISE
			targetDur = ei.MissionInfo_EPIC
			shipName = "Extended Henerprise"
		}
	}

	fmt.Fprintf(b, "🎯 **Goal**: %s (`%s`)\n", getGoalLabel(goal), shipName)

	reqs := ei.GetMissionFuels(targetShip, targetDur, false)
	if len(reqs) == 0 {
		b.WriteString("No fuel requirements found for target ship.\n")
		return
	}

	// Calculate fuel per launch & launches possible
	minLaunchesPossible := math.MaxFloat64
	var bottleneckEgg ei.Egg
	var bottleneckStored, bottleneckNeeded float64

	b.WriteString("**Fuel Requirements & Status**:\n")

	var normalRoundingRiskEggs []string

	for _, req := range reqs {
		eggIdx := int(req.Egg) - 1
		currentStored := 0.0
		if eggIdx >= 0 && eggIdx < len(tankFuels) {
			currentStored = tankFuels[eggIdx]
		}

		eggEmoji, _ := ei.GetEggEmojiMarkdownIfExists(req.Egg)
		eggName := formatEggName(req.Egg)

		isFarmEgg := req.Egg == farmEgg
		if isFarmEgg {
			fmt.Fprintf(b, "- %s **%s**: Farm Egg (Continuous unlimited fuel) • Need %s/launch\n",
				eggEmoji, eggName, ei.FormatEIValue(req.Amount, map[string]any{"decimals": 1, "trim": true}))
			continue
		}

		launches := currentStored / req.Amount
		if launches < minLaunchesPossible {
			minLaunchesPossible = launches
			bottleneckEgg = req.Egg
			bottleneckStored = currentStored
			bottleneckNeeded = req.Amount
		}

		nLaunches := int(launches)
		exactCutoff := float64(nLaunches) * req.Amount
		isExact := nLaunches > 0 && isExactOrBorderlineFuel(currentStored, exactCutoff)
		exactNotice := ""
		if isExact {
			exactNotice = " ⚠️ *(Exact cutoff • rounding risk)*"
			normalRoundingRiskEggs = append(normalRoundingRiskEggs, formatEggName(req.Egg))
		}

		fmt.Fprintf(b, "- %s **%s**: %s in tank • Need %s/launch (Enough for **%d** launches)%s\n",
			eggEmoji, eggName,
			ei.FormatEIValue(currentStored, map[string]any{"decimals": 1, "trim": true}),
			ei.FormatEIValue(req.Amount, map[string]any{"decimals": 1, "trim": true}),
			nLaunches,
			exactNotice)
	}

	b.WriteString("\n")

	if len(normalRoundingRiskEggs) > 0 {
		fmt.Fprintf(b, "⚠️ **Fuel Tank Rounding Warning**: **%s** is exactly at the cutoff for its launch count with no buffer. Egg, Inc. often suffers from floating-point rounding errors when tank amounts are exact, causing the final launch to fail. Bank a slight buffer to avoid this issue!\n\n",
			strings.Join(normalRoundingRiskEggs, ", "))
	}

	// FUEL WARNING SECTION
	launchesRemaining := int(minLaunchesPossible)
	if minLaunchesPossible == math.MaxFloat64 {
		launchesRemaining = 0
	}

	if launchesRemaining == 0 {
		fmt.Fprintf(b, "🚨 **CRITICAL FUEL WARNING**: You are **OUT OF FUEL** for `%s`! Tank has insufficient **%s** (%s stored, %s needed per launch). Prestige and refill now!\n",
			shipName,
			formatEggName(bottleneckEgg),
			ei.FormatEIValue(bottleneckStored, map[string]any{"decimals": 1, "trim": true}),
			ei.FormatEIValue(bottleneckNeeded, map[string]any{"decimals": 1, "trim": true}))
	} else if launchesRemaining <= 2 {
		fmt.Fprintf(b, "⚠️ **FUEL WARNING**: Tank only has enough fuel for **%d** more launch(es) of `%s`! **%s** will run out first. Plan to prestige and refill soon!\n",
			launchesRemaining, shipName, formatEggName(bottleneckEgg))
	} else {
		fmt.Fprintf(b, "✅ **Fuel Healthy**: Stored fuel supports **%d** more `%s` launches before requiring a refill (bottleneck: %s).\n",
			launchesRemaining, shipName, formatEggName(bottleneckEgg))
	}
}

// renderEnlightenmentRunAdvisor gives specialized planning for an Enlightenment Egg run.
func renderEnlightenmentRunAdvisor(b *strings.Builder, backup *ei.Backup, tankCapacity float64) {
	b.WriteString("🎯 **Goal**: 🧘 **Maximize Enlightenment Egg Run**\n")
	b.WriteString("-# On the Enlightenment egg, egg value is 0 and no rocket fuel can be produced. All launches rely 100% on fuel stored in your tank!\n\n")

	// Extended Henliner: 2T Tachyon, 6T Dilithium, 6T Antimatter, 6T Dark Matter = 20T per ship
	targetShip := ei.MissionInfo_ATREGGIES
	shipName := "Extended Atreggies Henliner"
	fuelPerShip := 20e12
	reqs := ei.GetMissionFuels(targetShip, ei.MissionInfo_EPIC, false)

	if !hasUnlockedHenliner(backup) {
		targetShip = ei.MissionInfo_HENERPRISE
		shipName = "Extended Henerprise"
		fuelPerShip = 10e12
		reqs = ei.GetMissionFuels(targetShip, ei.MissionInfo_EPIC, false)
	}

	maxLaunches := int(math.Floor(tankCapacity / fuelPerShip))
	totalTriCycles := maxLaunches / 3

	// Estimate days lasting (Extended Henliner with FTL research)
	ftlMult := 1.0
	if backup.GetGame() != nil {
		ftlMult = ei.GetEpicResearchMissionTime(backup.GetGame().GetEpicResearch())
	}
	baseDurationDays := 4.0 // 4 days base for Extended Henliner
	cycleDays := baseDurationDays * ftlMult
	totalDays := float64(totalTriCycles) * cycleDays

	fmt.Fprintf(b, "**Tank Capacity**: %s\n", ei.FormatEIValue(tankCapacity, map[string]any{"decimals": 0, "trim": true}))
	fmt.Fprintf(b, "📦 **Max Full-Run Launches**: **%d launches** of `%s` (%d complete tri-launch cycles)\n",
		maxLaunches, shipName, totalTriCycles)
	fmt.Fprintf(b, "⏱️ **Estimated Mission Duration**: ~**%.1f days** of non-stop launches during Enlightenment\n\n", totalDays)

	b.WriteString("**Optimal Tank Fill Distribution**:\n")
	for _, req := range reqs {
		ratio := req.Amount / fuelPerShip
		targetFill := tankCapacity * ratio
		eggEmoji, _ := ei.GetEggEmojiMarkdownIfExists(req.Egg)
		eggName := formatEggName(req.Egg)
		fmt.Fprintf(b, "- %s **%s**: Fill to **%s** (%.0f%% of tank)\n",
			eggEmoji, eggName,
			ei.FormatEIValue(targetFill, map[string]any{"decimals": 1, "trim": true}),
			ratio*100)
	}
	b.WriteString("-# 💡 *Tip: Fill slightly under max for each egg so you don't accidentally overfill and lock out another fuel type!*\n")
}

// renderSundayDubcapAdvisor analyzes rocket return times relative to the next Sunday 48h Double Capacity event.
func renderSundayDubcapAdvisor(b *strings.Builder, activeMissions []*ei.MissionInfo, backup *ei.Backup) {
	b.WriteString("### 🚀 Sunday Double Capacity Event (48h) Timing\n")

	dubcapStart, dubcapEnd := getNextSundayEvent()
	now := time.Now().UTC()

	isCurrentlyActive := now.After(dubcapStart) && now.Before(dubcapEnd)

	dubcapIcon := ei.GetBotEmojiMarkdown("std_dubcap")
	if dubcapIcon == "" {
		dubcapIcon = "✨"
	}

	if isCurrentlyActive {
		fmt.Fprintf(b, "%s **Double Capacity Event is LIVE NOW!** Ends <t:%d:R> (<t:%d:f>)\n",
			dubcapIcon, dubcapEnd.Unix(), dubcapEnd.Unix())
	} else {
		fmt.Fprintf(b, "%s **Next Sunday Dubcap**: Starts <t:%d:f> (<t:%d:R>) • Ends <t:%d:f>\n",
			dubcapIcon, dubcapStart.Unix(), dubcapStart.Unix(), dubcapEnd.Unix())
	}

	if len(activeMissions) == 0 {
		b.WriteString("No active rockets deployed. You can schedule new launches freely to land within the Dubcap window!\n")
		return
	}

	ftlMult := 1.0
	if backup.GetGame() != nil {
		ftlMult = ei.GetEpicResearchMissionTime(backup.GetGame().GetEpicResearch())
	}

	b.WriteString("\n**Slot Landing Analysis**:\n")
	for i, m := range activeMissions {
		shipType := m.GetShip()
		shipName := ei.MissionInfo_Spaceship_name[int32(shipType)]
		if int(shipType) < len(ei.MissionArt.Ships) {
			shipName = ei.MissionArt.Ships[shipType].Name
		}

		returnSec := int64(m.GetStartTimeDerived() + m.GetDurationSeconds())
		if returnSec == 0 && m.GetSecondsRemaining() > 0 {
			returnSec = time.Now().Unix() + int64(m.GetSecondsRemaining())
		}
		returnTime := time.Unix(returnSec, 0).UTC()

		if returnTime.Before(dubcapStart) {
			// Lands BEFORE Sunday starts
			leadTime := dubcapStart.Sub(returnTime)
			fmt.Fprintf(b, "- **Slot %d** (`%s`): Lands <t:%d:R> (%.1fh before Dubcap)\n",
				i+1, shipName, returnSec, leadTime.Hours())

			// Check if a Short filler mission can be run
			shortDurationSec := 1200.0 // Default 20m
			if durParam, ok := ei.GetShipMissionParams(shipType, ei.MissionInfo_SHORT); ok && durParam.Seconds > 0 {
				shortDurationSec = durParam.Seconds
			}
			shortDuration := time.Duration(shortDurationSec*ftlMult) * time.Second

			if leadTime > shortDuration {
				fillerEnd := returnTime.Add(shortDuration)
				fmt.Fprintf(b, "  - 💡 *Recommendation: Run a **Short** filler mission next; lands at <t:%d:t>, right in time for Dubcap!*\n",
					fillerEnd.Unix())
			} else {
				b.WriteString("  - 🎯 *Recommendation: Ready for immediate Extended launch once Dubcap begins!*\n")
			}
		} else if returnTime.Before(dubcapEnd) {
			// Lands DURING the 48h event window
			timeLeftInDubcap := dubcapEnd.Sub(returnTime)
			fmt.Fprintf(b, "- **Slot %d** (`%s`): Lands <t:%d:R> (**DURING DUBCAP!** 🎉)\n",
				i+1, shipName, returnSec)

			exHenSec := 4.0 * 86400 * ftlMult
			if timeLeftInDubcap > time.Duration(exHenSec)*time.Second {
				b.WriteString("  - 🌟 *Amazing timing: Launch an Extended mission right away; it will return in time for a **2nd Dubcap launch**!*\n")
			} else {
				fmt.Fprintf(b, "  - 🚀 *Launch your best Extended mission before event ends <t:%d:R> to claim 2x capacity!*\n", dubcapEnd.Unix())
			}
		} else {
			// Lands AFTER event ends
			fmt.Fprintf(b, "- **Slot %d** (`%s`): Lands <t:%d:R> (After Dubcap ends ⚠️)\n",
				i+1, shipName, returnSec)
		}
	}
}

// getNextSundayEvent calculates the next Sunday 9 AM Pacific event window (lasting 48 hours).
func getNextSundayEvent() (start time.Time, end time.Time) {
	now := time.Now().UTC()
	pacificLoc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		pacificLoc = time.FixedZone("PDT", -7*3600)
	}

	nowPacific := now.In(pacificLoc)
	daysUntilSunday := (int(time.Sunday) - int(nowPacific.Weekday()) + 7) % 7

	targetDate := time.Date(nowPacific.Year(), nowPacific.Month(), nowPacific.Day(), 9, 0, 0, 0, pacificLoc).AddDate(0, 0, daysUntilSunday)
	eventEnd := targetDate.Add(48 * time.Hour)

	if nowPacific.After(eventEnd) {
		targetDate = targetDate.AddDate(0, 0, 7)
		eventEnd = targetDate.Add(48 * time.Hour)
	}

	return targetDate.UTC(), eventEnd.UTC()
}

// hasUnlockedHenliner checks if the player has unlocked Atreggies Henliner.
func hasUnlockedHenliner(backup *ei.Backup) bool {
	if backup == nil || backup.GetArtifactsDb() == nil {
		return false
	}
	allMissions := append(backup.GetArtifactsDb().GetMissionArchive(), backup.GetArtifactsDb().GetMissionInfos()...)
	for _, m := range allMissions {
		if m != nil && m.GetShip() == ei.MissionInfo_ATREGGIES {
			return true
		}
	}
	return false
}

// findNextAllStarsShip identifies which ship to launch for fastest All Stars Club progress.
func findNextAllStarsShip(backup *ei.Backup, virtue bool) (string, ei.MissionInfo_Spaceship, ei.MissionInfo_DurationType) {
	if backup == nil || backup.GetArtifactsDb() == nil {
		return "Short Atreggies Henliner", ei.MissionInfo_ATREGGIES, ei.MissionInfo_SHORT
	}

	allMissions := append(backup.GetArtifactsDb().GetMissionArchive(), backup.GetArtifactsDb().GetMissionInfos()...)
	var shipLP [11]float64
	for _, mi := range allMissions {
		if mi == nil || mi.GetStatus() < ei.MissionInfo_EXPLORING {
			continue
		}
		s := int(mi.GetShip())
		if s >= 0 && s < 11 {
			switch mi.GetDurationType() {
			case ei.MissionInfo_SHORT, ei.MissionInfo_TUTORIAL:
				shipLP[s] += 1.0
			case ei.MissionInfo_LONG:
				shipLP[s] += 1.4
			case ei.MissionInfo_EPIC:
				shipLP[s] += 1.8
			default:
				shipLP[s] += 1.0
			}
		}
	}

	// Find the ship with the least remaining LP to max that is not yet maxed
	minRemainingLP := math.MaxFloat64
	bestShip := ei.MissionInfo_ATREGGIES

	for s := 0; s < 11; s++ {
		maxLP := ei.ShipMaxLaunchPoints[s]
		if shipLP[s] < maxLP {
			remaining := maxLP - shipLP[s]
			if remaining < minRemainingLP {
				minRemainingLP = remaining
				bestShip = ei.MissionInfo_Spaceship(s)
			}
		}
	}

	shipName := ei.MissionInfo_Spaceship_name[int32(bestShip)]
	if int(bestShip) < len(ei.MissionArt.Ships) {
		shipName = ei.MissionArt.Ships[bestShip].Name
	}

	return fmt.Sprintf("Short %s (%.1f LP needed)", shipName, minRemainingLP), bestShip, ei.MissionInfo_SHORT
}

func getGoalLabel(goal string) string {
	switch goal {
	case GoalArtifactHunt:
		return "🏆 Artifact Hunting (Best Ships)"
	case GoalFuelEfficiency:
		return "📦 Most Artifacts / Used Fuel (Extended Henerprise)"
	case GoalAllStars:
		return "⭐ All Stars Club (Fastest LP)"
	case GoalEnlightenment:
		return "🧘 Maximize Enlightenment Run"
	default:
		return "🏆 Artifact Hunting"
	}
}

func getTargetArtifact(m *ei.MissionInfo) (ei.ArtifactSpec_Name, bool) {
	if m == nil {
		return 0, false
	}
	if m.TargetArtifact != nil && *m.TargetArtifact != ei.ArtifactSpec_UNKNOWN {
		return *m.TargetArtifact, true
	}
	if m.GetTargetArtifact() != 0 && m.GetTargetArtifact() != ei.ArtifactSpec_UNKNOWN {
		return m.GetTargetArtifact(), true
	}
	return 0, false
}

func getTargetArtifactName(target ei.ArtifactSpec_Name) string {
	if name, ok := ei.ArtifactTypeName[int32(target)]; ok && name != "" {
		return name
	}
	s := target.String()
	s = strings.TrimPrefix(s, "ArtifactSpec_")
	s = strings.ReplaceAll(s, "_", " ")
	return s
}

func getTargetArtifactEmoji(target ei.ArtifactSpec_Name) string {
	candidates := getTargetEmojiCandidates(target)
	for _, cand := range candidates {
		if md, ok := ei.GetBotEmojiMarkdownIfExists(cand); ok {
			return md
		}
	}
	if len(candidates) > 0 {
		md := ei.GetBotEmojiMarkdown(candidates[0])
		if md != "" && md != "<::>" && !strings.Contains(md, "unknown") {
			return md
		}
	}
	return ""
}

func getTargetEmojiCandidates(target ei.ArtifactSpec_Name) []string {
	switch target {
	case ei.ArtifactSpec_TACHYON_STONE, ei.ArtifactSpec_TACHYON_STONE_FRAGMENT:
		return []string{"st_T4", "afx_tachyon_stone_4", "TACHYON_T4C", "TACHYON_4"}
	case ei.ArtifactSpec_QUANTUM_STONE, ei.ArtifactSpec_QUANTUM_STONE_FRAGMENT:
		return []string{"st_Q4", "afx_quantum_stone_4", "QUANTUM_T4C", "QUANTUM_4"}
	case ei.ArtifactSpec_LIFE_STONE, ei.ArtifactSpec_LIFE_STONE_FRAGMENT:
		return []string{"st_Li4", "afx_life_stone_4", "LIFE_T4C", "LIFE_4"}
	case ei.ArtifactSpec_LUNAR_STONE, ei.ArtifactSpec_LUNAR_STONE_FRAGMENT:
		return []string{"st_Lu4", "afx_lunar_stone_4", "LUNAR_T4C", "LUNAR_4"}
	case ei.ArtifactSpec_DILITHIUM_STONE, ei.ArtifactSpec_DILITHIUM_STONE_FRAGMENT:
		return []string{"st_Di4", "DILITHIUM_T4C", "DILITHIUM_4"}
	case ei.ArtifactSpec_SHELL_STONE, ei.ArtifactSpec_SHELL_STONE_FRAGMENT:
		return []string{"st_Sh4", "SHELL_T4C", "SHELL_4"}
	case ei.ArtifactSpec_SOUL_STONE, ei.ArtifactSpec_SOUL_STONE_FRAGMENT:
		return []string{"st_So4", "SOUL_T4C", "SOUL_4"}
	case ei.ArtifactSpec_PROPHECY_STONE, ei.ArtifactSpec_PROPHECY_STONE_FRAGMENT:
		return []string{"st_Pr4", "PROPHECY_T4C", "PROPHECY_4"}
	case ei.ArtifactSpec_CLARITY_STONE, ei.ArtifactSpec_CLARITY_STONE_FRAGMENT:
		return []string{"st_Cl4", "CLARITY_T4C", "CLARITY_4"}
	case ei.ArtifactSpec_TERRA_STONE, ei.ArtifactSpec_TERRA_STONE_FRAGMENT:
		return []string{"st_Te4", "TERRA_T4C", "TERRA_4"}
	case ei.ArtifactSpec_GOLD_METEORITE:
		return []string{"afx_gold_meteorite_3", "GOLD_T3C", "GOLD_T3", "GOLD_3"}
	case ei.ArtifactSpec_TAU_CETI_GEODE:
		return []string{"afx_tau_ceti_geode_3", "GEODE_T3C", "GEODE_T3", "GEODE_3"}
	case ei.ArtifactSpec_SOLAR_TITANIUM:
		return []string{"afx_solar_titanium_3", "TITANIUM_T3C", "TITANIUM_T3", "TITANIUM_3"}
	}

	if short, ok := ei.ShortArtifactName[int32(target)]; ok && short != "" {
		return []string{
			fmt.Sprintf("%sT4C", short),
			fmt.Sprintf("%sT4", short),
			fmt.Sprintf("%s4", short),
			fmt.Sprintf("%sT3C", short),
		}
	}
	return nil
}
