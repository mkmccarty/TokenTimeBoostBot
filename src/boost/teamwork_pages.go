package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/rs/xid"
)

type teamworkCache struct {
	xid                 string
	msgID               string
	header              string
	footer              string
	showScores          bool
	showingSIAB         bool
	page                int
	previousPage        int
	pages               int
	expirationTimestamp time.Time
	contractID          string
	coopID              string
	public              bool
	names               []string
	fields              map[string][]TeamworkOutputData
	scorefields         map[string]discordgo.MessageComponent
	siabField           []TeamworkOutputData
}

var teamworkCacheMap = make(map[string]teamworkCache)

// buildTeamworkCache will build a cache of the teamwork data
func buildTeamworkCache(s string, fields map[string][]TeamworkOutputData) teamworkCache {

	// Extract SIAB Fields from the fields map
	var siabFields []TeamworkOutputData
	if field, ok := fields["siab"]; ok {
		siabFields = field
	}
	delete(fields, "siab")

	// Extract and sort the keys from the fields map
	var keys []string
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Initialize the scorefields map
	scoreFields := make(map[string]discordgo.MessageComponent)

	// Traverse the fields map and look for the field with Name "Contract Score"
	for key, fieldList := range fields {
		for _, field := range fieldList {
			// Check if the field is a TextDisplay or a container with components
			// MessageComponent
			//   Container
			//     TextDisplay
			//     TextDisplay
			mycolor := 0xffaa00
			if field.Title == "Contract Score" {
				scoreFields[key] = discordgo.Container{
					AccentColor: &mycolor,
					Components: []discordgo.MessageComponent{
						discordgo.TextDisplay{
							Content: field.Title,
						},
						discordgo.TextDisplay{
							Content: field.Content,
						},
					},
				}
			}
		}
	}

	return teamworkCache{
		xid:                 xid.New().String(),
		header:              s,
		footer:              "",
		page:                0,
		previousPage:        0,
		pages:               len(fields),
		expirationTimestamp: time.Now().Add(24 * time.Hour),
		names:               keys,
		fields:              fields,
		scorefields:         scoreFields,
		siabField:           siabFields,
	}
}

func sendTeamworkPage(s *discordgo.Session, i *discordgo.InteractionCreate, newMessage bool, xid string, refresh bool, toggle bool, siabDisplay bool, drawButtons bool) {
	cache, exists := teamworkCacheMap[xid]

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

	if exists && (refresh || cache.expirationTimestamp.Before(time.Now())) {

		s1, fields, _ := DownloadCoopStatusTeamwork(cache.contractID, cache.coopID, 0)
		newCache := buildTeamworkCache(s1, fields)

		newCache.public = cache.public
		newCache.previousPage = cache.previousPage
		newCache.xid = cache.xid
		newCache.contractID = cache.contractID
		newCache.coopID = cache.coopID
		newCache.page = cache.page
		newCache.showScores = cache.showScores
		newCache.showingSIAB = cache.showingSIAB
		siabDisplay = newCache.showingSIAB
		if refresh {
			newCache.page = cache.previousPage
		}
		cache = newCache
		teamworkCacheMap[cache.xid] = newCache
	}

	if !exists {
		str := fmt.Sprintf("The teamwork data has expired. Please re-run the %s command.", bottools.GetFormattedCommand("teamwork"))
		comp := []discordgo.MessageComponent{}
		comp = append(comp, discordgo.TextDisplay{
			Content: str,
		})

		d2 := discordgo.WebhookEdit{
			Components: &comp,
		}

		_, err := s.FollowupMessageEdit(i.Interaction, i.Message.ID, &d2)
		if err != nil {
			log.Println(err)
		}

		/*
			time.AfterFunc(10*time.Second, func() {
				err := s.FollowupMessageDelete(i.Interaction, i.Message.ID)
				if err != nil {
					log.Println(err)
				}
			})
		*/
		return
	}

	if toggle {
		cache.showScores = !cache.showScores
		teamworkCacheMap[cache.xid] = cache
	}
	if siabDisplay != cache.showingSIAB {
		cache.showingSIAB = siabDisplay
		teamworkCacheMap[cache.xid] = cache
	}

	// if Refresh this should be the previous page
	if siabDisplay {
		cache.page = cache.previousPage
	}

	flags := discordgo.MessageFlagsEphemeral
	if cache.public {
		flags = 0
	}

	if cache.page < 0 || cache.page >= cache.pages {
		cache.page = 0
	}

	var comp []discordgo.MessageComponent
	comp = append(comp, discordgo.TextDisplay{
		Content: cache.header,
	})

	if len(cache.names) != 0 {
		key := cache.names[cache.page]
		field := cache.fields[key]

		if !siabDisplay {
			var container discordgo.Container
			// Need to make a component list of the field data.
			// First element of this is th player name
			var bodyText []discordgo.MessageComponent
			bodyText = append(bodyText, discordgo.TextDisplay{
				Content: "## " + field[0].Content,
			})
			for _, f := range field[1:] {
				// Section header - should be a Label but that's not in the library yet.
				bodyText = append(bodyText, discordgo.TextDisplay{
					Content: "### " + f.Title,
				})
				// Section data.
				bodyText = append(bodyText, discordgo.TextDisplay{
					Content: f.Content,
				})
			}

			myColor := 0xffaa00
			container = discordgo.Container{
				Components:  bodyText,
				AccentColor: &myColor,
			}
			comp = append(comp, container)

		} else {
			var container discordgo.Container
			// Need to make a component list of the field data.
			// First element of this is th player name
			var bodyText []discordgo.MessageComponent
			bodyText = append(bodyText, discordgo.TextDisplay{
				Content: "## " + cache.siabField[0].Content,
			})
			for _, f := range cache.siabField[1:] {
				// Section header - should be a Label but that's not in the library yet.
				bodyText = append(bodyText, discordgo.TextDisplay{
					Content: "### " + f.Title,
				})
				// Section data.
				bodyText = append(bodyText, discordgo.TextDisplay{
					Content: f.Content,
				})
			}

			myColor := 0xffaa00
			container = discordgo.Container{
				Components:  bodyText,
				AccentColor: &myColor,
			}
			comp = append(comp, container)
		}
	}
	if newMessage {

		if drawButtons {
			comp = append(comp, getTeamworkComponents(cache.xid, cache.page, cache.pages)...)
		}

		msg, err := s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Flags:      flags | discordgo.MessageFlagsIsComponentsV2,
				Components: comp,
			})
		if err != nil {
			log.Println(err)
		} else {
			cache.msgID = msg.ID
		}

	} else {
		if drawButtons {
			comp = append(comp, getTeamworkComponents(cache.xid, cache.page, cache.pages)...)
		}

		d2 := discordgo.WebhookEdit{
			Flags:      flags | discordgo.MessageFlagsIsComponentsV2,
			Components: &comp,
		}

		_, err := s.FollowupMessageEdit(i.Interaction, i.Message.ID, &d2)
		if err != nil {
			log.Println(err)
		}
	}

	// Don't advance the page if we're on the siabDisplay
	cache.previousPage = cache.page
	if !siabDisplay {
		cache.page = cache.page + 1
		if cache.page >= cache.pages {
			cache.page = 0
		}
	}

	teamworkCacheMap[cache.xid] = cache
}

// HandleTeamworkPage steps a page of cached teamwork data
func HandleTeamworkPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// cs_#Name # cs_#ID # HASH
	refresh := false
	siabSelection := false
	toggle := false
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})

	drawButtons := true
	if err != nil {
		log.Println(err)
	}
	if len(reaction) == 3 && reaction[2] == "refresh" {
		refresh = true
	}
	if len(reaction) == 3 && reaction[2] == "toggle" {
		toggle = true
	}
	if len(reaction) == 3 && reaction[2] == "siab" {
		siabSelection = true
	}
	if len(reaction) == 3 && reaction[2] == "close" {
		drawButtons = false
		delete(teamworkCacheMap, reaction[1])
	}
	sendTeamworkPage(s, i, false, reaction[1], refresh, toggle, siabSelection, drawButtons)

	if drawButtons == false {
		delete(teamworkCacheMap, reaction[1])
	}
}

// getTokenValComponents returns the components for the token value
func getTeamworkComponents(name string, page int, pageEnd int) []discordgo.MessageComponent {
	var buttons []discordgo.Button

	if pageEnd != 0 {
		buttons = append(buttons, discordgo.Button{
			Label:    fmt.Sprintf("Page %d/%d", page+1, pageEnd),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_teamwork#%s", name),
		})
	}
	buttons = append(buttons,
		discordgo.Button{
			Label:    "Refresh",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_teamwork#%s#refresh", name),
		})
	/*
		buttons = append(buttons,
			discordgo.Button{
				Label:    "Scores Toggle",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("fd_teamwork#%s#toggle", name),
			})
	*/
	buttons = append(buttons,
		discordgo.Button{
			Label:    "SIAB Swap Times",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_teamwork#%s#siab", name),
		})
	buttons = append(buttons,
		discordgo.Button{
			Label:    "Close",
			Style:    discordgo.DangerButton,
			CustomID: fmt.Sprintf("fd_teamwork#%s#close", name),
		})

	var components []discordgo.MessageComponent
	for _, button := range buttons {
		components = append(components, button)
	}
	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: components}}
}
