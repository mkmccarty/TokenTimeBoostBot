package boost

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// buildStonesCache will build a cache of the stones data
func buildStonesCache(s string, url string, tiles []dc.EmbedField) stonesCache {
	// Split string by "```" characters into a header, body and footer
	split := strings.Split(s, "```")

	// Split by lines
	table := strings.Split(split[1], "\n")
	var trimmedTable []string
	trimmedTable = append(trimmedTable, table...)
	table = trimmedTable
	tableHeader := table[0] + "\n"
	table = table[1:]

	return stonesCache{uuidStr: uuid.NewV7().String(), header: split[0], footer: split[2], tableHeader: tableHeader, table: table, page: 0, pages: len(table) / 10, expirationTimestamp: time.Now().Add(15 * time.Minute), url: url, tiles: tiles}
}

// sendStonesPage renders one page of a cached stones report.
//
// It still takes a raw session because the boost list redraw is not on the
// facade yet.
func sendStonesPage(client dc.Client, e dc.InteractionEvent, newMessage bool, uuidStr string, refresh bool, links bool, toggle bool) {
	stonesCacheMutex.Lock()
	cache, exists := stonesCacheMap[uuidStr]
	stonesCacheMutex.Unlock()

	if exists && links && cache.url != "" {

		if time.Now().Before(cache.LinkTime) && len(cache.urlPages) == 1 {
			return
		}

		if len(cache.urlPages) == 0 {
			var pageBuilder strings.Builder
			var currentPageSize int

			for line := range strings.SplitSeq(cache.url, "\n") {
				lineSize := len(line) + 1 // +1 for the newline character
				if currentPageSize+lineSize > 1800 {
					cache.urlPages = append(cache.urlPages, pageBuilder.String())
					pageBuilder.Reset()
					currentPageSize = 0
				}
				pageBuilder.WriteString(line)
				pageBuilder.WriteString("\n")
				currentPageSize += lineSize
			}

			if pageBuilder.Len() > 0 {
				cache.urlPages = append(cache.urlPages, pageBuilder.String())
			}
		}

		err := e.Followup(dc.Message{
			Content:        fmt.Sprintf("## Staabmia's Stone Calculator Links (%d/%d)\n%s", cache.urlPage+1, len(cache.urlPages), cache.urlPages[cache.urlPage]),
			SuppressEmbeds: true,
			Ephemeral:      cache.private,
		})
		if err != nil {
			log.Println(err)
		}
		cache.urlPage++
		if cache.urlPage >= len(cache.urlPages) {
			cache.urlPage = 0
		}
		cache.LinkTime = time.Now().Add(1 * time.Minute)
		stonesCacheMutex.Lock()
		stonesCacheMap[uuidStr] = cache
		stonesCacheMutex.Unlock()
		return
	}
	_ = e.Followup(dc.Message{})

	if exists && (refresh || cache.expirationTimestamp.Before(time.Now())) {

		s1, urls, tiles := DownloadCoopStatusStones(e.ChannelID(), cache.contractID, cache.coopID, cache.details, cache.soloName, cache.useBuffHistory, cache.eiID)
		newCache := buildStonesCache(s1, urls, tiles)

		newCache.private = cache.private
		newCache.uuidStr = cache.uuidStr
		newCache.contractID = cache.contractID
		newCache.coopID = cache.coopID
		newCache.eiID = cache.eiID
		newCache.details = cache.details
		newCache.soloName = cache.soloName
		newCache.useBuffHistory = cache.useBuffHistory
		newCache.page = cache.page
		newCache.displayTiles = cache.displayTiles
		cache = newCache
		stonesCacheMutex.Lock()
		stonesCacheMap[cache.uuidStr] = newCache
		stonesCacheMutex.Unlock()

		contract := FindContractByIDs(e.ChannelID(), cache.contractID, cache.coopID)
		if contract != nil {
			if contract.State == ContractStateCompleted {
				// Only refresh if EstimateUpdateTime is within 10 seconds of now
				if math.Abs(time.Since(contract.EstimateUpdateTime).Seconds()) <= 10 {
					refreshBoostListMessage(client, contract, false)
				}
			}
		}

	}

	if !exists {

		str := fmt.Sprintf("The stones data has expired. Please re-run the %s command.\n", bottools.GetFormattedCommand("stones"))
		str += e.MessageContent()

		err := e.EditFollowup(e.MessageID(), dc.Message{
			Content:      str,
			ComponentsV1: true,
		})
		if err != nil {
			log.Println(err)
		}

		return
	}

	if toggle {
		cache.displayTiles = !cache.displayTiles
		stonesCacheMutex.Lock()
		stonesCacheMap[cache.uuidStr] = cache
		stonesCacheMutex.Unlock()
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
		itemsPerPage = 60
		if cache.page*itemsPerPage >= len(cache.table) {
			cache.page = 0
		}
		cache.pages = int(math.Ceil(float64(len(cache.table)) / float64(itemsPerPage)))
	}

	var field []dc.EmbedField
	var embed []dc.Embed

	page := cache.page

	start := page * itemsPerPage
	end := start + itemsPerPage
	if end > len(cache.table) {
		end = len(cache.table)
	}

	if !cache.displayTiles {
		var currentField strings.Builder
		currentField.WriteString(cache.tableHeader)
		for _, line := range cache.table[start:end] {
			if currentField.Len()+len(line)+1 > 950 { // +1 for the newline character
				field = append(field, dc.EmbedField{
					Name:  "",
					Value: currentField.String(),
				})
				currentField.Reset()
			}
			currentField.WriteString(line)
			currentField.WriteString("\n")
		}
		if currentField.Len() > 0 {
			field = append(field, dc.EmbedField{
				Name:  "",
				Value: currentField.String(),
			})
		}
		embed = []dc.Embed{{
			Title:       "Stones Report",
			Description: cache.header,
			Fields:      field,
			Footer:      &dc.EmbedFooter{Text: strings.ReplaceAll(cache.footer, "⭐️", "√")},
		}}
	} else {

		for i := start; i < end && i < len(cache.tiles); i++ {
			field = append(field, cache.tiles[i])
		}

		embed = []dc.Embed{{
			Title:       "Stones Report",
			Description: cache.header,
			Fields:      field,
			Footer:      &dc.EmbedFooter{Text: strings.ReplaceAll(cache.footer, "√", "⭐️")},
		}}

	}

	cache.page = page + 1

	// Content sits beside an ActionRow and embeds here, which is the legacy
	// component model.
	msg := dc.Message{
		Components:   getStonesComponents(cache.uuidStr, page, cache.pages),
		Embeds:       embed,
		ComponentsV1: true,
	}

	if newMessage {
		ref, err := e.FollowupMessage(msg)
		if err != nil {
			log.Println(err)
		} else {
			cache.msgID = ref.ID
		}

	} else {
		err := e.EditFollowup(e.MessageID(), msg)
		if err != nil {
			log.Println(err)
		}
	}
	stonesCacheMutex.Lock()
	stonesCacheMap[cache.uuidStr] = cache
	stonesCacheMutex.Unlock()
}

// HandleStonesPage steps a page of cached stones data through the dc facade.
//
// It still takes a raw session because sendStonesPage can trigger a boost list
// redraw, which is not on the facade yet.
func HandleStonesPage(client dc.Client, e *dc.ComponentEvent) {
	// cs_#Name # cs_#ID # HASH
	refresh := false
	links := false
	toggle := false
	reaction := strings.Split(e.CustomID(), "#")

	if len(reaction) == 3 && reaction[2] == "close" {
		stonesCacheMutex.Lock()
		delete(stonesCacheMap, reaction[1])
		stonesCacheMutex.Unlock()
	}

	err := e.DeferUpdate()
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
		return
	}

	sendStonesPage(client, e, false, reaction[1], refresh, links, toggle)
}

// getStonesComponents returns the components for the token value
func getStonesComponents(name string, page int, pageEnd int) []dc.LayoutComponent {
	var buttons []dc.InteractiveComponent

	if pageEnd > 1 {
		buttons = append(buttons, dc.Button{
			Label:    fmt.Sprintf("Page %d/%d", page+1, pageEnd),
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_stones#%s", name),
		})
	}
	buttons = append(buttons,
		dc.Button{
			Label:    "Refresh",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_stones#%s#refresh", name),
		})

	buttons = append(buttons,
		dc.Button{
			Label:    "Tile/Table",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("fd_stones#%s#toggle", name),
		})
	/*
		buttons = append(buttons,
			dc.Button{
				Label:    "staabmia links",
				Style:    dc.ButtonSecondary,
				CustomID: fmt.Sprintf("fd_stones#%s#links", name),
			})
	*/
	buttons = append(buttons,
		dc.Button{
			Label:    "Close",
			Style:    dc.ButtonDanger,
			CustomID: fmt.Sprintf("fd_stones#%s#close", name),
		})

	return []dc.LayoutComponent{dc.ActionRow{Components: buttons}}
}

type stonesCache struct {
	uuidStr             string
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
	eiID                string
	details             bool
	soloName            string
	useBuffHistory      bool
	url                 string
	urlPage             int
	urlPages            []string
	private             bool
	LinkTime            time.Time
	tiles               []dc.EmbedField
}

var (
	stonesCacheMap   = make(map[string]stonesCache)
	stonesCacheMutex sync.Mutex
)
