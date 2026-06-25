package events

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/notok"
	"github.com/mkmccarty/TokenTimeBoostBot/src/watch"
	"google.golang.org/protobuf/proto"
)

const expectedActiveContracts = 6
const expectedContractRoleNames = 30
const expectedContractComplaints = 12

const periodicalsLocationName = "America/Los_Angeles"

func hasExpectedActiveContracts(contracts []ei.EggIncContract) bool {
	activeContracts := 0
	for _, contract := range contracts {
		if contract.Predicted {
			continue
		}
		activeContracts++
	}
	return activeContracts >= expectedActiveContracts
}

func countExpectedActiveContracts(contracts []ei.EggIncContract) int {
	activeContracts := 0
	for _, contract := range contracts {
		if contract.Predicted {
			continue
		}
		activeContracts++
	}
	return activeContracts
}

func findEventsStartedToday(events []ei.EggEvent, now time.Time, loc *time.Location) []ei.EggEvent {
	nowLocal := now.In(loc)
	todayYear, todayMonth, todayDay := nowLocal.Date()

	var todayEvents []ei.EggEvent

	for _, event := range events {
		if event.StartTime.IsZero() {
			continue
		}

		eventLocal := event.StartTime.In(loc)
		y, m, d := eventLocal.Date()
		if y != todayYear || m != todayMonth || d != todayDay {
			continue
		}

		todayEvents = append(todayEvents, event)
	}

	sort.Slice(todayEvents, func(i, j int) bool {
		return todayEvents[i].StartTime.Before(todayEvents[j].StartTime)
	})

	return todayEvents
}

func findOngoingEvents(events []ei.EggEvent, now time.Time) []ei.EggEvent {
	var ongoing []ei.EggEvent

	for _, event := range events {
		if event.StartTime.IsZero() || event.EndTime.IsZero() {
			continue
		}
		if now.Before(event.StartTime) {
			continue
		}
		if !now.Before(event.EndTime) {
			continue
		}
		ongoing = append(ongoing, event)
	}

	sort.Slice(ongoing, func(i, j int) bool {
		return ongoing[i].EndTime.Before(ongoing[j].EndTime)
	})

	return ongoing
}

// HasEventsStartedToday returns all events that started today in Pacific Time.
func HasEventsStartedToday(events []ei.EggEvent, now time.Time) []ei.EggEvent {
	loc, err := time.LoadLocation(periodicalsLocationName)
	if err != nil {
		log.Printf("Error loading timezone %s: %v", periodicalsLocationName, err)
		loc = time.Local
	}

	return findEventsStartedToday(events, now, loc)
}

// HasOngoingEventsNow returns all events that are currently active.
func HasOngoingEventsNow(events []ei.EggEvent, now time.Time) []ei.EggEvent {
	return findOngoingEvents(events, now)
}

// HasEventStartedToday returns the latest event that started today in Pacific Time.
func HasEventStartedToday(events []ei.EggEvent, now time.Time) (ei.EggEvent, bool) {
	todayEvents := HasEventsStartedToday(events, now)
	if len(todayEvents) == 0 {
		return ei.EggEvent{}, false
	}

	return todayEvents[len(todayEvents)-1], true
}

func getAAAFinalSeasonCxpGoal(seasonInfo *ei.ContractSeasonInfo) float64 {
	if seasonInfo == nil {
		return 0
	}

	for _, goalSet := range seasonInfo.GetGradeGoals() {
		if goalSet == nil || goalSet.GetGrade() != ei.Contract_GRADE_AAA {
			continue
		}
		goals := goalSet.GetGoals()
		if len(goals) == 0 {
			continue
		}
		return goals[len(goals)-1].GetCxp()
	}

	return 0
}

// GetPeriodicalsFromAPI will download the events from the Egg Inc API.
// Returns true if it detects a meaningful live periodicals refresh.
func GetPeriodicalsFromAPI(s *discordgo.Session) bool {
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
		return false
	}
	values := url.Values{}
	reqDataEncoded := enc.EncodeToString(reqBin)
	values.Set("data", string(reqDataEncoded))

	response, err := http.PostForm(reqURL, values)
	if err != nil {
		log.Print(err)
		return false
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
		return false
	}

	protoData := string(body)

	decodedAuthBuf := &ei.AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return false
	}

	periodicalsResponse := &ei.PeriodicalsResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, periodicalsResponse)
	if err != nil {
		log.Print(err)
		return false
	}

	// Look for new events
	localEventMap := make(map[string]ei.EggEvent)
	for k, v := range ei.AllEventMap {
		localEventMap[k] = v
	}

	var currentEggIncEvents []ei.EggEvent
	newEvents := false

	newGG := 1.0
	newUltraGG := 1.0
	earningsEvent := 1.0
	earningsEventUltra := 1.0
	researchDiscountEvent := 1.0

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
		// log.Printf("  start time: %s", e.StartTime)

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
		if e.EventType == "earnings-boost" {
			if e.Ultra {
				earningsEventUltra = e.Multiplier
			} else {
				earningsEvent = e.Multiplier
			}
		}
		if e.EventType == "research-sale" {
			if !e.Ultra {
				researchDiscountEvent = e.Multiplier
			}
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
		ei.SortAndSwapEvents(localEventMap, currentEggIncEvents)
	}

	// Set our current Event variables
	ei.SetGenerousGiftEvent(newGG, newUltraGG, newEventEndGG)
	ei.SetEarningsEvent(earningsEvent, earningsEventUltra)
	ei.SetResearchDiscountEvent(researchDiscountEvent)

	changed := true

	notifyDiscordOfNewEgg := len(ei.CustomEggMap) != 0

	// Look for new Custom Eggs
	for _, customEgg := range periodicalsResponse.GetContracts().GetCustomEggs() {
		var egg ei.EggIncCustomEgg
		egg.ID = strings.ReplaceAll(customEgg.GetIdentifier(), " ", "")
		egg.Name = customEgg.GetName()
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
			watch.AddNewColleggtible(egg.ID)
			var builder strings.Builder
			// Do we have an icon for this egg?
			_, err := bottools.ImportEggImage(s, egg.ID, egg.IconURL)
			if err != nil {
				log.Print(err)
				// Can't continue here on error, so skip this egg
				continue
			}
			fmt.Fprintf(&builder, "New Custom Egg Detected: %s", egg.Name)

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
				acoChannel := "1257340301438222401" // ACO #colleggtibles-chat
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

	// Ensure the dynamic artifact map is up to date with the latest colleggtibles
	ei.PopulateColleggtiblesInArtifactMap()

	// Look for new contracts
	var newContract []ei.EggIncContract
	for _, contract := range periodicalsResponse.GetContracts().GetContracts() {
		c := boost.PopulateContractFromProto(contract)
		//log.Print("contract details: ", c.ID, " ", contract.GetCcOnly())
		// Time this record was imported from the periodicals API
		c.PeriodicalAPI = true

		// If we're reading a contract from a periodical then it's currently active
		// Check if the contract already exists and is the same
		existingContract, exists := ei.EggIncContractsAll[c.ID]
		if exists {
			if existingContract.ValidUntil != c.ValidUntil {
				log.Print("New Leggacy contract: ", c.ID)
			}
		} else if c.ID != "first-contract" {
			log.Print("New Original contract: ", c.ID)
			ei.EggIncContractsAll[c.ID] = c
		}
		bottools.GenerateBanner(c.ID, c.EggName, c.Name, "", "", "")
		existingTeamNames, err := boost.GetRoleNamesForContract(c.ID)
		if err != nil {
			log.Printf("periodicals: failed to load role names for %s: %v", c.ID, err)
		}

		existingComplaints, err := boost.GetThematicComplaintsForContract(c.ID)
		if err != nil {
			log.Printf("periodicals: failed to load complaints for %s: %v", c.ID, err)
		}

		needTeamNames := len(existingTeamNames) < expectedContractRoleNames
		needComplaints := len(existingComplaints) < expectedContractComplaints

		if len(existingTeamNames) > 0 {
			c.TeamNames = existingTeamNames
		}

		if needTeamNames || needComplaints {
			go fetchThematicDataAsync(c.ID, c.Name, c.Description, needTeamNames, needComplaints)
		}

		ei.EggIncContractsAll[c.ID] = c
		if c.ID != "first-contract" {
			newContract = append(newContract, c)
		}
	}

	// Set current season
	seasonInfo := periodicalsResponse.GetContracts().GetCurrentSeason()
	if seasonInfo != nil {
		ei.SetEggIncCurrentSeason(seasonInfo.GetId(), seasonInfo.GetName(), seasonInfo.GetStartTime())
		ei.SetEggIncCurrentSeasonAAAFinalCxpGoal(getAAAFinalSeasonCxpGoal(seasonInfo))
	}

	// Replace all new contracts
	if len(newContract) > 0 {
		for _, predicted := range boost.CreatePredictedContract() {
			newContract = append(newContract, predicted)
			ei.EggIncContractsAll[predicted.ID] = predicted
		}
		ei.EggIncContracts = newContract
	}

	if changed {
		saveCustomEggData(ei.CustomEggMap)
	}

	updatedPredicted := boost.UpdatePredictedSignupContracts(s, newContract)
	if updatedPredicted > 0 {
		log.Printf("Updated %d predicted signup contract(s) to live contract IDs", updatedPredicted)
	}

	now := time.Now()
	activeContractCount := countExpectedActiveContracts(newContract)
	todayEvents := HasEventsStartedToday(currentEggIncEvents, now)
	hasTodayEvent := len(todayEvents) > 0
	if hasTodayEvent {
		log.Printf("Today's periodical events (%d):", len(todayEvents))
		for idx, event := range todayEvents {
			log.Printf("  [%d/%d] type=%s ultra=%t multiplier=%.2f start=%s end=%s message=%q",
				idx+1,
				len(todayEvents),
				event.EventType,
				event.Ultra,
				event.Multiplier,
				event.StartTime.In(time.Local).Format(time.RFC3339),
				event.EndTime.In(time.Local).Format(time.RFC3339),
				event.Message,
			)
		}
	} else {
		log.Printf("No event found that started today. Active contracts=%d/%d", activeContractCount, expectedActiveContracts)
	}

	ongoingEvents := HasOngoingEventsNow(currentEggIncEvents, now)
	ongoingNotToday := make([]ei.EggEvent, 0, len(ongoingEvents))
	for _, event := range ongoingEvents {
		eventStartedToday := false
		for _, todayEvent := range todayEvents {
			if todayEvent.ID == event.ID {
				eventStartedToday = true
				break
			}
		}
		if !eventStartedToday {
			ongoingNotToday = append(ongoingNotToday, event)
		}
	}

	if len(ongoingNotToday) > 0 {
		log.Printf("Other ongoing periodical events (%d):", len(ongoingNotToday))
		for idx, event := range ongoingNotToday {
			log.Printf("  [%d/%d] type=%s ultra=%t multiplier=%.2f start=%s end=%s message=%q",
				idx+1,
				len(ongoingNotToday),
				event.EventType,
				event.Ultra,
				event.Multiplier,
				event.StartTime.In(time.Local).Format(time.RFC3339),
				event.EndTime.In(time.Local).Format(time.RFC3339),
				event.Message,
			)
		}
	} else {
		log.Print("No other ongoing periodical events right now.")
	}

	periodicalsReady := activeContractCount >= expectedActiveContracts && hasTodayEvent
	if !periodicalsReady {
		log.Printf("Periodicals not ready yet: active_contracts=%d/%d event_started_today=%t",
			activeContractCount,
			expectedActiveContracts,
			hasTodayEvent,
		)
	}

	go watch.CheckWatches(s)

	return periodicalsReady || newEvents || updatedPredicted > 0
}

var asyncPeriodicalMutex sync.Mutex

func fetchThematicDataAsync(contractID string, contractName string, contractDescription string, needTeamNames bool, needComplaints bool) {
	var teamNames []string
	var complaints []string

	if needTeamNames {
		teamNames = notok.GetContractTeamNames(contractDescription, expectedContractRoleNames)
	}
	if needComplaints {
		complaints = notok.GetContractThematicComplaints(contractName, contractDescription, expectedContractComplaints)
	}

	if len(teamNames) > 0 || len(complaints) > 0 {
		asyncPeriodicalMutex.Lock()
		defer asyncPeriodicalMutex.Unlock()

		if len(teamNames) > 0 {
			boost.SaveRoleNames(map[string][]string{contractID: teamNames})
			ei.SetContractTeamNames(contractID, teamNames)
		}

		if len(complaints) > 0 {
			_ = boost.SaveThematicComplaints(map[string][]string{contractID: complaints})
			boost.PopulateThematicComplaintsForContractID(contractID, complaints)
		}
	}
}
