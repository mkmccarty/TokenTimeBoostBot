package boost

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/rs/xid"
)

// buildStonesCache will build a cache of the stones data
func buildStonesCache(s string, url string, tiles []*discordgo.MessageEmbedField) stonesCache {
	// Split string by "```" characters into a header, body and footer
	split := strings.Split(s, "```")

	table := strings.Split(split[1], "\n")
	var trimmedTable []string
	for _, line := range table {
		if strings.TrimSpace(line) != "" {
			trimmedTable = append(trimmedTable, line)
		}
	}
	table = trimmedTable
	tableHeader := table[0] + "\n"
	table = table[1:]

	return stonesCache{xid: xid.New().String(), header: split[0], footer: split[2], tableHeader: tableHeader, table: table, page: 0, pages: len(table) / 10, expirationTimestamp: time.Now().Add(15 * time.Minute), url: url, tiles: tiles}
}

func sendStonesPage(s *discordgo.Session, i *discordgo.InteractionCreate, newMessage bool, xid string, refresh bool, links bool, toggle bool) {
	cache, exists := stonesCacheMap[xid]

	if exists && links && cache.url != "" {

		if time.Now().Before(cache.LinkTime) && len(cache.urlPages) == 1 {
			return
		}
		flags := discordgo.MessageFlagsSupressEmbeds
		if cache.private {
			flags += discordgo.MessageFlagsEphemeral
		}

		if len(cache.urlPages) == 0 {
			var pageBuilder strings.Builder
			var currentPageSize int

			for _, line := range strings.Split(cache.url, "\n") {
				lineSize := len(line) + 1 // +1 for the newline character
				if currentPageSize+lineSize > 1800 {
					cache.urlPages = append(cache.urlPages, pageBuilder.String())
					pageBuilder.Reset()
					currentPageSize = 0
				}
				pageBuilder.WriteString(line + "\n")
				currentPageSize += lineSize
			}

			if pageBuilder.Len() > 0 {
				cache.urlPages = append(cache.urlPages, pageBuilder.String())
			}
		}

		_, err := s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: fmt.Sprintf("## Staabmia's Stone Calculator Links (%d/%d)\n%s", cache.urlPage+1, len(cache.urlPages), cache.urlPages[cache.urlPage]),
				Flags:   flags,
			})
		if err != nil {
			log.Println(err)
		}
		cache.urlPage++
		if cache.urlPage >= len(cache.urlPages) {
			cache.urlPage = 0
		}
		cache.LinkTime = time.Now().Add(1 * time.Minute)
		stonesCacheMap[xid] = cache
		return
	}
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{})

	if exists && (refresh || cache.expirationTimestamp.Before(time.Now())) {

		s1, urls, tiles := DownloadCoopStatusStones(cache.contractID, cache.coopID, cache.details, cache.soloName, cache.useBuffHistory)
		newCache := buildStonesCache(s1, urls, tiles)

		newCache.private = cache.private
		newCache.xid = cache.xid
		newCache.contractID = cache.contractID
		newCache.coopID = cache.coopID
		newCache.details = cache.details
		newCache.soloName = cache.soloName
		newCache.useBuffHistory = cache.useBuffHistory
		newCache.page = cache.page
		newCache.displayTiles = cache.displayTiles
		cache = newCache
		stonesCacheMap[cache.xid] = newCache

		//delete(stonesCacheMap, xid)
		//exists = false
	}

	if !exists {

		str := fmt.Sprintf("The stones data has expired. Please re-run the %s command.\n", bottools.GetFormattedCommand("stones"))
		str += (*(*(*i).Interaction).Message).Content
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
		cache.displayTiles = !cache.displayTiles
		stonesCacheMap[cache.xid] = cache
	}

	// if Refresh this should be the previous page
	if refresh || toggle {
		cache.page = cache.page - 1
		if cache.page < 0 {
			cache.page = 0
		}
	}

	var itemsPerPage int

	if cache.displayTiles {
		itemsPerPage = 12
		if cache.page*itemsPerPage >= len(cache.tiles) {
			cache.page = 0
		}
		cache.pages = int(math.Ceil(float64(len(cache.table)) / float64(itemsPerPage)))

	} else {
		itemsPerPage = 10
		if cache.page*itemsPerPage >= len(cache.table) {
			cache.page = 0
		}
		cache.pages = int(math.Ceil(float64(len(cache.table)) / float64(itemsPerPage)))
	}

	var flags discordgo.MessageFlags

	field := []*discordgo.MessageEmbedField{}
	var embed []*discordgo.MessageEmbed

	page := cache.page
	var builder strings.Builder

	start := page * itemsPerPage
	end := start + itemsPerPage
	if end > len(cache.table) {
		end = len(cache.table)
	}

	if !cache.displayTiles {
		flags |= discordgo.MessageFlagsSuppressEmbeds
		builder.WriteString(cache.header)
		builder.WriteString("```")
		builder.WriteString(cache.tableHeader)

		for _, line := range cache.table[start:end] {
			builder.WriteString(line + "\n")
		}

		builder.WriteString("```")
		builder.WriteString(cache.footer)
	} else {

		field = append(field, cache.tiles[start:end]...)

		embed = []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			Title:       "Stones Report",
			Description: cache.header,
			Fields:      field,
			Footer:      &discordgo.MessageEmbedFooter{Text: strings.ReplaceAll(cache.footer, "√", "⭐️")},
		}}

	}

	cache.page = page + 1

	if newMessage {
		msg, err := s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content:    builder.String(),
				Flags:      flags,
				Components: getStonesComponents(cache.xid, page, cache.pages),
				Embeds:     embed,
			})
		if err != nil {
			log.Println(err)
		}
		cache.msgID = msg.ID

	} else {
		comp := getStonesComponents(cache.xid, page, cache.pages)

		str := builder.String()
		d2 := discordgo.WebhookEdit{
			Content:    &str,
			Components: &comp,
			Embeds:     &embed,
		}

		msg, err := s.FollowupMessageEdit(i.Interaction, i.Message.ID, &d2)
		if err != nil {
			log.Println(err)
		}
		log.Print(msg.ID)
	}
	stonesCacheMap[cache.xid] = cache
}

// HandleStonesPage steps a page of cached stones data
func HandleStonesPage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// cs_#Name # cs_#ID # HASH
	refresh := false
	links := false
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
	if len(reaction) == 3 && reaction[2] == "links" {
		links = true
	}
	if len(reaction) == 3 && reaction[2] == "toggle" {
		toggle = true
	}
	if len(reaction) == 3 && reaction[2] == "close" {
		delete(stonesCacheMap, reaction[1])
	}

	sendStonesPage(s, i, false, reaction[1], refresh, links, toggle)
}

// getTokenValComponents returns the components for the token value
func getStonesComponents(name string, page int, pageEnd int) []discordgo.MessageComponent {
	var buttons []discordgo.Button

	if pageEnd != 0 {
		buttons = append(buttons, discordgo.Button{
			Label:    fmt.Sprintf("Page %d/%d", page+1, pageEnd),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_stones#%s", name),
		})
	}
	buttons = append(buttons,
		discordgo.Button{
			Label:    "Refresh",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_stones#%s#refresh", name),
		})
	/*
		buttons = append(buttons,
			discordgo.Button{
				Label:    "Tile/Table",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("fd_stones#%s#toggle", name),
			})
	*/
	buttons = append(buttons,
		discordgo.Button{
			Label:    "staabmia links",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("fd_stones#%s#links", name),
		})

	buttons = append(buttons,
		discordgo.Button{
			Label:    "Close",
			Style:    discordgo.DangerButton,
			CustomID: fmt.Sprintf("fd_stones#%s#close", name),
		})

	var components []discordgo.MessageComponent
	for _, button := range buttons {
		components = append(components, button)
	}
	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: components}}
}

type stonesCache struct {
	xid                 string
	msgID               string
	displayTiles        bool
	header              string
	footer              string
	tableHeader         string
	table               []string
	page                int
	pages               int
	expirationTimestamp time.Time
	contractID          string
	coopID              string
	details             bool
	soloName            string
	useBuffHistory      bool
	url                 string
	urlPage             int
	urlPages            []string
	private             bool
	LinkTime            time.Time
	tiles               []*discordgo.MessageEmbedField
}

var stonesCacheMap = make(map[string]stonesCache)
