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
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	values := url.Values{}
	reqDataEncoded := enc.EncodeToString(reqBin)
	values.Set("data", string(reqDataEncoded))

	response, err := http.PostForm(reqURL, values)
	if err != nil {
		log.Print(err)
		return
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

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

	newGG := 1.0
	newUltraGG := 1.0
	var newEventEndGG time.Time

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
		if e.EventType == "gift-boost" {
			if e.Ultra {
				newUltraGG = e.Multiplier
			} else {
				newGG = e.Multiplier
			}
			newEventEndGG = e.EndTime
		}

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

	// Set our current Event variables
	ei.SetGenerousGiftEvent(newGG, newUltraGG, newEventEndGG)

	/*
		// Look for new Custom Eggs
		ei.CustomEggMap, err = LoadCustomEggData()
		if err != nil {
			ei.CustomEggMap = make(map[string]*ei.EggIncCustomEgg)
		}
	*/
	changed := true
	c := cases.Title(language.Und)

	notifyDiscordOfNewEgg := len(ei.CustomEggMap) != 0

	// Look for new Custom Eggs
	for _, customEgg := range periodicalsResponse.GetContracts().GetCustomEggs() {
		var egg ei.EggIncCustomEgg
		egg.ID = strings.ReplaceAll(customEgg.GetIdentifier(), " ", "")
		egg.Name = c.String(customEgg.GetName())
		egg.Value = customEgg.GetValue()
		egg.IconName = customEgg.GetIcon().GetName()
		egg.IconURL = customEgg.GetIcon().GetUrl()
		egg.IconWidth = int(customEgg.GetIconWidth())
		egg.IconHeight = int(customEgg.GetIconHeight())
		for _, d := range customEgg.GetBuffs() {
			egg.Dimension = d.GetDimension()
			egg.DimensionName = ei.GetGameDimensionString(d.GetDimension())
			egg.DimensionValue = append(egg.DimensionValue, d.GetValue())
			if d.GetValue() >= 2.0 {
				egg.DimensionValueString = append(egg.DimensionValueString, fmt.Sprintf("%1.0fx", d.GetValue()))
			} else if d.GetValue() > 1.0 {
				egg.DimensionValueString = append(egg.DimensionValueString, fmt.Sprintf("+%1.0f%%", (d.GetValue()-1.0)*100.0))

			} else {
				egg.DimensionValueString = append(egg.DimensionValueString, fmt.Sprintf("%1.0f%%", d.GetValue()*100.0))
			}
		}

		if len(egg.DimensionValueString) > 0 {
			egg.Description = "Up to " + egg.DimensionValueString[len(egg.DimensionValueString)-1] + " " + egg.DimensionName
			egg.Description += fmt.Sprintf("\nValue: %g", egg.Value)
		}

		eggProtoBin, _ := proto.Marshal(customEgg)
		egg.Proto = base64.StdEncoding.EncodeToString(eggProtoBin)

		if _, exists := ei.CustomEggMap[egg.ID]; exists {
			if ei.CustomEggMap[egg.ID].Proto == egg.Proto {
				continue
			}
		} else if notifyDiscordOfNewEgg {
			var builder strings.Builder
			// Do we have an icon for this egg?
			builder.WriteString(fmt.Sprintf("New Custom Egg Detected: %s", egg.Name))
			_, err := bottools.ImportEggImage(s, egg.ID, egg.IconURL)
			if err != nil {
				log.Print(err)
				// Can't continue here on error, so skip this egg
				continue
			}

			description := strings.Join(egg.DimensionValueString, ",") + " " + egg.DimensionName
			description += fmt.Sprintf("\n%s Value: %g", ei.GetBotEmojiMarkdown(egg.ID), egg.Value)
			// Send a message about a new egg
			u, _ := s.UserChannelCreate(config.AdminUserID)
			var data discordgo.MessageSend
			data.Content = builder.String()
			data.Embed = &discordgo.MessageEmbed{
				Title:       egg.Name,
				Description: description,
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: egg.IconURL,
				},
			}
			_, err = s.ChannelMessageSendComplex(u.ID, &data)
			if err != nil {
				log.Print(err)
			}

			// Also send this for ACO
			if !config.IsDevBot() {
				acoChannel := "1103074428352471050" // ACO #contracts-version-2-chat
				permissions, err := s.UserChannelPermissions(config.DiscordAppID, acoChannel)
				if err != nil {
					log.Printf("Error getting permissions for channel %s: %v", acoChannel, err)
				} else {
					if permissions&discordgo.PermissionSendMessages == 0 {
						log.Printf("Bot does not have permission to send messages in channel %s", acoChannel)
					} else {
						_, err = s.ChannelMessageSendComplex(acoChannel, &data)
						if err != nil {
							log.Print(err)
						}
					}
				}
			}
		}

		ei.CustomEggMap[egg.ID] = &egg
		ei.SetColleggtibleValues()
		changed = true
	}

	// Look for new contracts
	var newContract []ei.EggIncContract
	for _, contract := range periodicalsResponse.GetContracts().GetContracts() {
		c := boost.PopulateContractFromProto(contract)
		if c.ID == "first-contract" {
			continue
		}
		//log.Print("contract details: ", c.ID, " ", contract.GetCcOnly())
		// Time this record was imported from the periodicals API
		c.PeriodicalAPI = true

		// If we're reading a contract from a periodical then it's currently active
		// Check if the contract already exists and is the same
		existingContract, exists := ei.EggIncContractsAll[c.ID]
		if exists {
			if existingContract.ExpirationTime != c.ExpirationTime {
				log.Print("New Leggacy contract: ", c.ID)
			}
		} else {
			log.Print("New Original contract: ", c.ID)
		}
		bottools.GenerateBanner(c.ID, c.EggName, c.Name)
		ei.EggIncContractsAll[c.ID] = c
		newContract = append(newContract, c)
	}

	// Replace all new contracts
	if len(newContract) > 0 {
		ei.EggIncContracts = newContract
	}

	if changed {
		saveCustomEggData(ei.CustomEggMap)
	}
}
