package menno

import (
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// DefaultMinimumDrops is the default minimum drops to use if none is set
const DefaultMinimumDrops = 1000

// SlashHuntCommand returns the command for the /hunt command
func SlashHuntCommand(cmd string) *dc.Command {
	zero, maxStars := 0, 8
	var shipChoices []dc.Choice[int]

	for i := 0; i < len(ei.ShipTypeName); i++ {
		shipChoices = append(shipChoices, dc.Choice[int]{
			Name:  ei.ShipTypeName[int32(i)],
			Value: i,
		})
	}
	// Create a duration type choice list
	var durationTypeChoices []dc.Choice[int]
	// Collect, sort, and build choices in ascending key order
	for i := 0; i < len(ei.DurationTypeName); i++ {
		durationTypeChoices = append(durationTypeChoices, dc.Choice[int]{
			Name:  ei.DurationTypeName[int32(i)],
			Value: i,
		})
	}
	commandOne := []dc.Option{
		dc.IntOption{
			Name:        "ship",
			Description: "Select the ship to search",
			Required:    true,
			Choices:     shipChoices,
		},
		dc.IntOption{
			Name:        "duration-type",
			Description: "Select the duration type",
			Required:    true,
			Choices:     durationTypeChoices,
		},
		dc.StringOption{
			Name:         "artifact",
			Description:  "What artifact or ingredient to hunt, searchable",
			Required:     true,
			Autocomplete: true,
		},
		dc.IntOption{
			Name:        "stars",
			Description: "Ship star level, default max",
			MinValue:    &zero,
			MaxValue:    &maxStars,
			Required:    false,
		},
		dc.IntOption{
			Name:        "minimum-drops",
			Description: "Select the minimum number of drops (Sticky)",
			MinValue:    &zero,
			Required:    false,
		},
	}

	commandTwo := []dc.Option{
		dc.StringOption{
			Name:         "artifact",
			Description:  "What artifact or ingredient to hunt, searchable",
			Required:     true,
			Autocomplete: true,
		},
		dc.IntOption{
			Name:        "duration-type",
			Description: "Select the duration type (Sticky)",
			Required:    false,
			Choices:     durationTypeChoices,
		},
		dc.IntOption{
			Name:        "minimum-drops",
			Description: "Select the minimum number of drops (Sticky)",
			MinValue:    &zero,
			Required:    false,
		},
	}

	command := dc.Command{
		Name:        cmd,
		Description: "Find artifact drop probabilities",
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
			dc.SubCommand{
				Name:        "ship",
				Description: "Custom single ship hunt of the Menno drop data",
				Options:     commandOne,
			},
			dc.SubCommand{
				Name:        "item",
				Description: "Hunt Menno drop data across multiple ships",
				Options:     commandTwo,
			},
		},
	}
	return &command
}

// HandleHuntAutocomplete handles the autocomplete for the /hunt command through
// the dc facade.
func HandleHuntAutocomplete(e *dc.AutocompleteEvent) {
	name, value := e.FocusedOption()
	searchString := ""
	if name == "artifact" {
		searchString = value
	}
	choices := make([]dc.Choice[string], 0)

	if searchString == "" {
		// No search string, start with a list of popular artifacts
		for id, name := range ei.ArtifactTypeNameVirtue {
			choices = append(choices, dc.Choice[string]{
				Name:  name,
				Value: fmt.Sprintf("%d", id),
			})
			if len(choices) >= 10 {
				break
			}
		}

		sort.Slice(choices, func(i, j int) bool {
			return choices[i].Name < choices[j].Name
		})

		if err := e.RespondChoices(choices); err != nil {
			fmt.Printf("HandleHuntAutoComplete InteractionRespond error: %v\n", err)
		}
		return
	}

	for id, name := range ei.ArtifactTypeName {
		if strings.Contains(strings.ToLower(name), strings.ToLower(searchString)) ||
			strings.Contains(strings.ToLower(fmt.Sprint(id)), strings.ToLower(searchString)) {

			choices = append(choices, dc.Choice[string]{
				Name:  name,
				Value: fmt.Sprintf("%d", id),
			})
			if len(choices) >= 10 {
				break
			}
		}
	}

	sort.Slice(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})

	_ = e.RespondChoices(choices)
}

// HandleHunt handles the /hunt command through the dc facade.
//
// It still takes a raw session for ei.GetFirstContactFromAPI, which is not on
// the facade yet.
func HandleHunt(e *dc.CommandEvent) {
	var response string
	artifactID := 10000 // No Target
	minimumDrops := DefaultMinimumDrops
	userID := e.UserID()

	subcommand, _ := e.Subcommand()
	ephemeral := false

	if subcommand == "ship" {
		shipID, _ := e.OptInt("ship-ship")
		shipStars := 8
		durationTypeID, _ := e.OptInt("ship-duration-type")
		if opt, ok := e.OptString("ship-artifact"); ok {
			artifactID, _ = strconv.Atoi(opt)
		}
		if opt, ok := e.OptInt("ship-stars"); ok {
			shipStars = opt
		}
		if opt, ok := e.OptInt("ship-minimum-drops"); ok {
			minimumDrops = opt
			farmerstate.SetMiscSettingString(userID, "huntMinimumDrops", fmt.Sprintf("%d", minimumDrops))
		} else {
			savedMinDrops := farmerstate.GetMiscSettingString(userID, "huntMinimumDrops")
			if savedMinDrops != "" {
				parsedMinDrops, err := strconv.Atoi(savedMinDrops)
				// GOOD: Check bounds before assigning to minimumDrops
				if err == nil && parsedMinDrops >= 0 && parsedMinDrops <= math.MaxInt32 {
					minimumDrops = parsedMinDrops
				} else {
					minimumDrops = DefaultMinimumDrops
				}
			}
		}
		_ = e.Defer(false)

		response = PrintDropData(ei.MissionInfo_Spaceship(shipID), ei.MissionInfo_DurationType(durationTypeID), shipStars, ei.ArtifactSpec_Name(artifactID), int32(minimumDrops))
	}

	if subcommand == "item" {
		// This command requires the user to be registered
		eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if eiID != "" {
			durationTypeID := 0
			if opt, ok := e.OptString("item-artifact"); ok {
				artifactID, _ = strconv.Atoi(opt)
			}
			if opt, ok := e.OptInt("item-duration-type"); ok {
				durationTypeID = opt
				farmerstate.SetMiscSettingString(userID, "huntItemDuration", fmt.Sprintf("%d", durationTypeID))
			} else {
				durationTypeStr := farmerstate.GetMiscSettingString(userID, "huntItemDuration")
				if durationTypeStr != "" {
					durationTypeID, _ = strconv.Atoi(durationTypeStr)
				}
			}
			if opt, ok := e.OptInt("item-minimum-drops"); ok {
				minimumDrops = opt
				farmerstate.SetMiscSettingString(userID, "huntMinimumDrops", fmt.Sprintf("%d", minimumDrops))
			} else {
				savedMinDrops := farmerstate.GetMiscSettingString(userID, "huntMinimumDrops")
				if savedMinDrops != "" {
					minimumDrops, _ = strconv.Atoi(savedMinDrops)
				}
			}

			eggIncID := ""
			encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
			if err == nil {
				decodedData, err := base64.StdEncoding.DecodeString(eiID)
				if err == nil {
					decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
					if err == nil {
						eggIncID = string(decryptedData)
					}
				}
			}
			_ = e.Defer(false)

			backup, _ := ei.GetFirstContactFromAPI(eggIncID, userID, true)

			response = PrintUserDropData(backup, ei.MissionInfo_DurationType(durationTypeID), ei.ArtifactSpec_Name(artifactID), int32(minimumDrops))
		} else {
			ephemeral = true
			response = fmt.Sprintf("You must register your EI ID with the bot to use this command. Use the %s command.", bottools.GetFormattedCommand("register"))

			_ = e.Respond(dc.Message{
				Ephemeral:  ephemeral,
				Components: []dc.LayoutComponent{dc.TextDisplay{Content: response}},
			})
			return
		}
	}

	err := e.Followup(dc.Message{
		Ephemeral:  ephemeral,
		Components: []dc.LayoutComponent{dc.TextDisplay{Content: response}},
	})
	if err != nil {
		fmt.Printf("HandleHuntCommand error: %v\n", err)
	}

}
