package boost

import (
	"encoding/base64"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"google.golang.org/protobuf/proto"
)

// GetEggIncEvents will download the events from the Egg Inc API
func GetEggIncEvents() {
	//userID := "EI6374748324102144"
	//userID := "EI5086937666289664"
	userID := "EI5410012152725504"
	//userID := config.EIUserID
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

}
