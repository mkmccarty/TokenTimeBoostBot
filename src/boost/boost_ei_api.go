package boost

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"google.golang.org/protobuf/proto"
)

func downloadCoopStatus(contractID string, coopID string) {

	userID := "EI6374748324102144"
	reqURL := "https://www.auxbrain.com/ei/coop_status"

	coopStatusRequest := ei.ContractCoopStatusRequest{
		ContractIdentifier: &contractID,
		CoopIdentifier:     &coopID,
		UserId:             &userID,
	}
	enc := base64.StdEncoding
	reqBin, err := proto.Marshal(&coopStatusRequest)
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

	decodedAuthBuf := &ei.AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(string(body))
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)

	decodeCoopStatus := &ei.ContractCoopStatusResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, decodeCoopStatus)

	for _, c := range decodeCoopStatus.GetContributors() {
		name := c.GetUserName()
		for _, a := range c.GetFarmInfo().GetEquippedArtifacts() {
			log.Println(name, " Artifact: ", a.GetSpec(), " ", a.GetStones())

		}
	}
	log.Print(decodeCoopStatus)
}
