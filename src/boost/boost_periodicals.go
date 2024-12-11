package boost

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"google.golang.org/protobuf/proto"
)

// GetEggIncEvents will download the events from the Egg Inc API
func GetEggIncEvents() {
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
	for _, event := range periodicalsResponse.GetEvents().GetEvents() {
		log.Print("event details: ")
		log.Printf("  type: %s", event.GetType())
		log.Printf("  text: %s", event.GetSubtitle())
		log.Printf("  multiplier: %f", event.GetMultiplier())

		startTimestamp := int64(math.Round(event.GetStartTime()))
		startTime := time.Unix(startTimestamp, 0)
		endTime := startTime.Add(time.Duration(event.GetDuration()) * time.Second)
		log.Printf("  start time: %s", startTime)
		log.Printf("  end time: %s", endTime)

		log.Printf("ultra: %t", event.GetCcOnly())

	}

	// Look for new contracts

	for _, contract := range periodicalsResponse.GetContracts().GetContracts() {
		var c ei.EggIncContract

		// Create a protobuf for the contract
		contractBin, _ := proto.Marshal(contract)
		c.ID = contract.GetIdentifier()
		c.Proto = base64.StdEncoding.EncodeToString(contractBin)

		// Print the ID and the fist 32 bytes of the c.Proto
		log.Print("contract details: ", c.ID, " ", contract.GetCcOnly())
	}

	// Look for new Custom Eggs

	eggMap, err := loadCustomEggData()
	if err != nil {
		eggMap = make(map[string]*ei.EggIncCustomEgg)
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

		if _, exists := eggMap[egg.ID]; exists {
			// If the proto is the same, skip
			if eggMap[egg.ID].Proto == egg.Proto {
				continue
			}
		}

		log.Print("custom egg details: ", egg.ID, "  ", egg.Proto[:32])

		eggMap[egg.ID] = &egg
		changed = true
	}

	if changed {
		saveCustomEggData(eggMap)
	}
	log.Print("done")
}

func saveCustomEggData(c map[string]*ei.EggIncCustomEgg) {
	b, _ := json.Marshal(c)
	_ = dataStore.Write("ei-customeggs", b)
}

func loadCustomEggData() (map[string]*ei.EggIncCustomEgg, error) {
	var c map[string]*ei.EggIncCustomEgg
	b, err := dataStore.Read("ei-customeggs")
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}

	return c, nil
}
