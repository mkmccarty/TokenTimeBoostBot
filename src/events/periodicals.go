package events

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"google.golang.org/protobuf/proto"
)

// GetPeriodicalsFromAPI will download the events from the Egg Inc API
func GetPeriodicalsFromAPI(s *discordgo.Session) {
	userID := config.EIUserID
	reqURL := "https://www.auxbrain.com/ei/get_periodicals"
	enc := base64.StdEncoding
	clientVersion := uint32(99)

	periodicalsRequest := ei.GetPeriodicalsRequest{
		UserId:               &userID,
		CurrentClientVersion: &clientVersion,
	}
	reqBin, err := proto.Marshal(&periodicalsRequest)
	if err != nil {
		log.Print(err)
		return
	}
	reqDataEncoded := enc.EncodeToString(reqBin)
	response, err := http.PostForm(reqURL, url.Values{"data": {reqDataEncoded}})

	if err != nil {
		log.Print(err)
		return
	}

	defer response.Body.Close()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Print(err)
		return
	}

	protoData := string(body)

	decodedAuthBuf := &ei.AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return
	}

	periodicalsResponse := &ei.PeriodicalsResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, periodicalsResponse)
	if err != nil {
		log.Print(err)
		return
	}

	// Look for new events
	localEventMap := make(map[string]ei.EggEvent)
	for k, v := range AllEventMap {
		localEventMap[k] = v
	}

	var currentEggIncEvents []ei.EggEvent
	newEvents := false

	for _, event := range periodicalsResponse.GetEvents().GetEvents() {
		var e ei.EggEvent
		e.ID = event.GetIdentifier()
		e.EventType = event.GetType()
		e.Message = event.GetSubtitle()
		e.Multiplier = event.GetMultiplier()
		e.Ultra = event.GetCcOnly()
		e.StartTimestamp = event.GetStartTime()
		e.EndTimestamp = event.GetStartTime() + event.GetDuration()
		e.StartTime = time.Unix(int64(math.Round(event.GetStartTime())), 0)
		e.EndTime = e.StartTime.Add(time.Duration(event.GetDuration()) * time.Second)
		log.Printf("  start time: %s", e.StartTime)

		currentEggIncEvents = append(currentEggIncEvents, e)

		// Want to add the ultra extension to the event type so only unique events are kept
		name := e.EventType
		if e.Ultra {
			name += "-ultra"
		}
		if localEventMap[name].ID != e.ID {
			localEventMap[name] = e
			newEvents = true
			log.Print("event details: ")
			log.Printf("  type: %s", event.GetType())
			log.Printf("  text: %s", event.GetSubtitle())
			log.Printf("  multiplier: %f", event.GetMultiplier())
		}
	}

	if newEvents {
		sortAndSwapEvents(localEventMap, currentEggIncEvents)
	}

	// Look for new contracts
	var newContract []ei.EggIncContract
	for _, contract := range periodicalsResponse.GetContracts().GetContracts() {
		c := boost.PopulateContractFromProto(contract)
		if c.ID == "first-contract" {
			continue
		}
		log.Print("contract details: ", c.ID, " ", contract.GetCcOnly())
		// Time this record was imported from the periodicals API
		c.PeriodicalAPI = true

		// If we're reading a contract from a periodical then it's currently active
		ei.EggIncContractsAll[c.ID] = c
		newContract = append(newContract, c)
	}

	// Replace all new contracts
	if len(newContract) > 0 {
		ei.EggIncContracts = newContract
	}

	// Look for new Custom Eggs
	ei.CustomEggMap, err = loadCustomEggData()
	if err != nil {
		ei.CustomEggMap = make(map[string]*ei.EggIncCustomEgg)
	}
	changed := false
	// Look for new Custom Eggs
	for _, customEgg := range periodicalsResponse.GetContracts().GetCustomEggs() {
		var egg ei.EggIncCustomEgg
		egg.ID = customEgg.GetIdentifier()
		egg.Name = customEgg.GetName()
		egg.Value = customEgg.GetValue()
		egg.IconName = customEgg.GetIcon().GetName()
		egg.IconURL = customEgg.GetIcon().GetUrl()
		egg.IconWidth = int(customEgg.GetIconWidth())
		egg.IconHeight = int(customEgg.GetIconHeight())
		for _, d := range customEgg.GetBuffs() {
			egg.Dimension = d.GetDimension()
			egg.DimensionValue = append(egg.DimensionValue, d.GetValue())
		}

		eggProtoBin, _ := proto.Marshal(customEgg)
		egg.Proto = base64.StdEncoding.EncodeToString(eggProtoBin)

		if _, exists := ei.CustomEggMap[egg.ID]; exists {
			if ei.CustomEggMap[egg.ID].Proto == egg.Proto {
				continue
			}
		} else {
			var builder strings.Builder
			builder.WriteString(fmt.Sprintf("New Custom Egg Detected: %s", egg.Name))

			u, _ := s.UserChannelCreate(config.AdminUserID)
			var data discordgo.MessageSend
			data.Content = builder.String()
			data.Embed = &discordgo.MessageEmbed{
				Title:       egg.Name,
				Description: fmt.Sprintf("%s %s\nValue: %f", ei.GetGameDimensionString(egg.Dimension), float64SliceToStringSlice(egg.DimensionValue), egg.Value),
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: egg.IconURL,
				},
			}

			_, err := s.ChannelMessageSendComplex(u.ID, &data)
			if err != nil {
				log.Print(err)
			}
		}

		log.Print("custom egg details: ", egg.ID, "  ", egg.Proto[:32])

		ei.CustomEggMap[egg.ID] = &egg
		changed = true
	}

	if changed {
		saveCustomEggData(ei.CustomEggMap)
	}
}

func float64SliceToStringSlice(slice []float64) string {
	strSlice := make([]string, len(slice))
	for i, v := range slice {
		strSlice[i] = fmt.Sprintf("%2.2f", v)
	}
	return strings.Join(strSlice, ", ")
}
