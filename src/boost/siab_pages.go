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

type siabCache struct {
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
	fields              map[string][]*discordgo.MessageEmbedField
	scorefields         map[string]*discordgo.MessageEmbedField
	siabField           []*discordgo.MessageEmbedField
}

var siabCacheMap = make(map[string]siabCache)

// buildSiabCache will build a cache of the teamwork data
func buildSiabCache(s string, fields map[string][]*discordgo.MessageEmbedField) siabCache {

	// Extract SIAB Fields from the fields map
	var siabFields []*discordgo.MessageEmbedField
	if field, ok := fields["siab"]; ok {
		// Check if the Value of the first field is greater than 900 bytes
		if len(field[0].Value) > 700 {
			// Split the Value by line breaks
			lines := strings.Split(field[0].Value, "\n")
			var currentField strings.Builder
			var fieldCount int
			for _, line := range lines {
				if currentField.Len()+len(line)+1 > 700 {
					siabFields = append(siabFields, &discordgo.MessageEmbedField{Name: "", Value: currentField.String(), Inline: false})
					currentField.Reset()
					fieldCount++
				}
				currentField.WriteString(line + "\n")
			}
			// Add the last field if it's not empty
			if currentField.Len() > 0 {
				siabFields = append(siabFields, &discordgo.MessageEmbedField{Name: "", Value: currentField.String(), Inline: false})
			}
		} else {
			field[0].Name = ""
			siabFields = field
		}
	}
	delete(fields, "siab")

	// Extract and sort the keys from the fields map
	var keys []string
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Initialize the scorefields map
	scoreFields := make(map[string]*discordgo.MessageEmbedField)

	// Traverse the fields map and look for the field with Name "Contract Score"
	for key, fieldList := range fields {
		for _, field := range fieldList {
			if field.Name == "Contract Score" {
				scoreFields[key] = field
			}
		}
	}

	return siabCache{
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

func sendSiabPage(s *discordgo.Session, i *discordgo.InteractionCreate, newMessage bool, xid string, refresh bool, toggle bool, siabDisplay bool) {
	cache, exists := siabCacheMap[xid]

	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

	if exists && (refresh || cache.expirationTimestamp.Before(time.Now())) {

		s1, _, _ := DownloadCoopStatusTeamwork(cache.contractID, cache.coopID, 0)
		//newCache := buildSiabCache(s1, fields)
		newCache := buildSiabCache(s1, nil)

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
		siabCacheMap[cache.xid] = newCache
	}

	if !exists {
		str := fmt.Sprintf("The teamwork data has expired. Please re-run the %s command.", bottools.GetFormattedCommand("teamwork"))
		comp := []discordgo.MessageComponent{}
		d2 := discordgo.WebhookEdit{
			Content:    &str,
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
		siabCacheMap[cache.xid] = cache
	}
	if siabDisplay != cache.showingSIAB {
		cache.showingSIAB = siabDisplay
		siabCacheMap[cache.xid] = cache
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

	key := cache.names[cache.page]

	field := cache.fields[key]

	var embed *discordgo.MessageSend

	if !siabDisplay {
		embed = &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{{
				Type:        discordgo.EmbedTypeRich,
				Title:       fmt.Sprintf("%s Teamwork Evaluation", field[0].Value),
				Description: "",
				Color:       0xffaa00,
				Fields:      field[1:],
				Timestamp:   time.Now().Format(time.RFC3339),
			}},
		}
	} else {

		embed = &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{{
				Type:        discordgo.EmbedTypeRich,
				Title:       "Equip SIAB Until...",
				Description: "",
				Color:       0xffaa00,
				Fields:      cache.siabField,
				Timestamp:   time.Now().Format(time.RFC3339),
			}},
		}
	}

	if newMessage {
		msg, err := s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content:    cache.header,
				Flags:      flags,
				Components: getSiabComponents(cache.xid, cache.page, cache.pages),
				Embeds:     embed.Embeds,
			})
		if err != nil {
			log.Println(err)
		}
		cache.msgID = msg.ID

	} else {
		comp := getSiabComponents(cache.xid, cache.page, cache.pages)

		str := cache.header
		d2 := discordgo.WebhookEdit{
			Content:    &str,
			Components: &comp,
			Embeds:     &embed.Embeds,
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

	siabCacheMap[cache.xid] = cache
}

// HandleSiabPage steps a page of cached siab data
func HandleSiabPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
		delete(siabCacheMap, reaction[1])
	}
	sendSiabPage(s, i, false, reaction[1], refresh, toggle, siabSelection)
}

// getTokenValComponents returns the components for the token value
func getSiabComponents(name string, page int, pageEnd int) []discordgo.MessageComponent {
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
