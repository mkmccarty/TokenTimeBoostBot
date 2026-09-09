package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

type teamworkCache struct {
	xid                 string
	msgID               string
	header              string
	footer              string
	showScores          bool
	page                int
	previousPage        int
	pages               int
	expirationTimestamp time.Time
	contractID          string
	coopID              string
	eiID                string
	public              bool
	names               []string
	fields              map[string][]TeamworkOutputData
	scorefields         map[string]dc.LayoutComponent
}

var teamworkCacheMap = make(map[string]teamworkCache)

// buildTeamworkCache will build a cache of the teamwork data
func buildTeamworkCache(s string, fields map[string][]TeamworkOutputData) teamworkCache {

	// Extract and sort the keys from the fields map
	var keys []string
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Initialize the scorefields map
	scoreFields := make(map[string]dc.LayoutComponent)

	// Traverse the fields map and look for the field with Name "Contract Score"
	for key, fieldList := range fields {
		for _, field := range fieldList {
			// Check if the field is a TextDisplay or a container with components
			// LayoutComponent
			//   Container
			//     TextDisplay
			//     TextDisplay
			if field.Title == "Contract Score" {
				scoreFields[key] = dc.Container{
					AccentColor: 0xffaa00,
					Components: []dc.ContainerSubComponent{
						dc.TextDisplay{
							Content: field.Title,
						},
						dc.TextDisplay{
							Content: field.Content,
						},
					},
				}
			}
		}
	}

	return teamworkCache{
		xid:                 uuid.NewV7().String(),
		header:              s,
		footer:              "",
		page:                0,
		previousPage:        0,
		pages:               len(fields),
		expirationTimestamp: time.Now().Add(24 * time.Hour),
		names:               keys,
		fields:              fields,
		scorefields:         scoreFields,
	}
}

func sendTeamworkPage(e dc.InteractionEvent, newMessage bool, xid string, refresh bool, toggle bool, drawButtons bool) {
	cache, exists := teamworkCacheMap[xid]

	_ = e.Followup(dc.Message{})

	if exists && (refresh || cache.expirationTimestamp.Before(time.Now())) {

		s1, fields, _ := DownloadCoopStatusTeamwork(e.ChannelID(), cache.contractID, cache.coopID, true, cache.eiID)
		newCache := buildTeamworkCache(s1, fields)

		newCache.public = cache.public
		newCache.previousPage = cache.previousPage
		newCache.xid = cache.xid
		newCache.contractID = cache.contractID
		newCache.coopID = cache.coopID
		newCache.eiID = cache.eiID
		newCache.page = cache.page
		newCache.showScores = cache.showScores
		if refresh {
			newCache.page = cache.previousPage
		}
		cache = newCache
		teamworkCacheMap[cache.xid] = newCache
	}

	if !exists {
		str := fmt.Sprintf("The teamwork data has expired. Please re-run the %s command.", bottools.GetFormattedCommand("teamwork"))

		err := e.EditFollowup(e.MessageID(), dc.Message{
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: str},
			},
		})
		if err != nil {
			log.Println(err)
		}

		return
	}

	if toggle {
		cache.showScores = !cache.showScores
		teamworkCacheMap[cache.xid] = cache
	}

	ephemeral := !cache.public

	if cache.page < 0 || cache.page >= cache.pages {
		cache.page = 0
	}

	var comp []dc.LayoutComponent
	comp = append(comp, dc.TextDisplay{
		Content: cache.header,
	})

	if len(cache.names) != 0 {
		key := cache.names[cache.page]
		field := cache.fields[key]
		// Need to make a component list of the field data.
		// First element of this is th player name
		var bodyText []dc.ContainerSubComponent
		bodyText = append(bodyText, dc.TextDisplay{
			Content: "## " + field[0].Content,
		})
		for _, f := range field[1:] {
			// Section header - should be a Label but that's not in the library yet.
			bodyText = append(bodyText, dc.TextDisplay{
				Content: fmt.Sprintf("### %s\n%s\n", f.Title, f.Content),
			})
		}

		comp = append(comp, dc.Container{
			Components:  bodyText,
			AccentColor: 0xffaa00,
		})
	}

	if drawButtons {
		comp = append(comp, getTeamworkComponents(cache.xid, cache.page, cache.pages)...)
	}

	if newMessage {
		msg, err := e.FollowupMessage(dc.Message{
			Ephemeral:  ephemeral,
			Components: comp,
		})
		if err != nil {
			log.Println(err)
		} else {
			cache.msgID = msg.ID
		}

	} else {
		err := e.EditFollowup(e.MessageID(), dc.Message{
			Ephemeral:  ephemeral,
			Components: comp,
		})
		if err != nil {
			log.Println(err)
		}
	}

	cache.previousPage = cache.page
	cache.page = cache.page + 1
	if cache.page >= cache.pages {
		cache.page = 0
	}

	teamworkCacheMap[cache.xid] = cache
}

// HandleTeamworkPage steps a page of cached teamwork data through the dc
// facade.
func HandleTeamworkPage(e *dc.ComponentEvent) {
	// cs_#Name # cs_#ID # HASH
	refresh := false
	toggle := false
	reaction := strings.Split(e.CustomID(), "#")

	err := e.DeferUpdate()

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
	if len(reaction) == 3 && reaction[2] == "close" {
		drawButtons = false
	}
	sendTeamworkPage(e, false, reaction[1], refresh, toggle, drawButtons)

	if !drawButtons {
		delete(teamworkCacheMap, reaction[1])
	}
}

// getTeamworkComponents returns the components for the token value
func getTeamworkComponents(name string, page int, pageEnd int) []dc.LayoutComponent {
	var buttons []dc.InteractiveComponent

	if pageEnd != 0 {
		buttons = append(buttons, dc.Button{
			Label:    fmt.Sprintf("Page %d/%d", page+1, pageEnd),
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_teamwork#%s", name),
		})
	}
	buttons = append(buttons,
		dc.Button{
			Label:    "Refresh",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_teamwork#%s#refresh", name),
		})
	/*
		buttons = append(buttons,
			dc.Button{
				Label:    "Scores Toggle",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("fd_teamwork#%s#toggle", name),
			})
	*/
	buttons = append(buttons,
		dc.Button{
			Label:    "Close",
			Style:    dc.ButtonDanger,
			CustomID: fmt.Sprintf("fd_teamwork#%s#close", name),
		})

	return []dc.LayoutComponent{dc.ActionRow{Components: buttons}}
}
