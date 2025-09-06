package ei

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"google.golang.org/protobuf/proto"
)

// GetFirstContactFromAPI will download the events from the Egg Inc API
func GetFirstContactFromAPI(s *discordgo.Session) {
	userID := config.EIUserIDBasic
	reqURL := "https://www.auxbrain.com//ei/bot_first_contact"
	enc := base64.StdEncoding
	clientVersion := uint32(68)

	platform := Platform_IOS
	platformString := "IOS"
	version := "1.34.1"
	build := "111300"

	firstContactRequest := EggIncFirstContactRequest{
		Rinfo: &BasicRequestInfo{
			EiUserId:      &userID,
			ClientVersion: &clientVersion,
			Version:       &version,
			Build:         &build,
			Platform:      &platformString,
		},
		EiUserId:      &userID,
		DeviceId:      proto.String("BoostBot"),
		ClientVersion: &clientVersion,
		Platform:      &platform,
	}

	reqBin, err := proto.Marshal(&firstContactRequest)
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
	rawDecodedText, _ := enc.DecodeString(protoData)
	firstContactResponse := &EggIncFirstContactResponse{}
	err = proto.Unmarshal(rawDecodedText, firstContactResponse)
	if err != nil {
		log.Print(err)
		return
	}

	backup := firstContactResponse.GetBackup()
	if backup == nil {
		log.Print("No backup found in Egg Inc API response")
		return
	}
	PE := backup.GetGame().GetEggsOfProphecy()
	SE := backup.GetGame().GetSoulEggsD()
	log.Printf("%s  \nPE:%v \nSE:%s\n", backup.GetUserName(), PE, FormatEIValue(SE, map[string]any{"decimals": 3, "trim": true}))

	archive := backup.GetContracts().GetArchive()

	// Want a function which will take an the archive parameter and print out the contractID, coopID, and cxp for each archived contract
	printArchivedContracts(archive)
}

func printArchivedContracts(archive []*LocalContract) {
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return
	}
	log.Printf("Downloaded %d archived contracts from Egg Inc API\n", len(archive))
	for _, c := range archive {
		contractID := c.GetContract().GetIdentifier()
		coopID := c.GetCoopIdentifier()
		evaluation := c.GetEvaluation()
		cxp := evaluation.GetCxp()
		// Print the values of contractID, coopID, and cxp
		log.Printf("Archived Contract ID: %s, Coop ID: %s, CXP: %f\n", contractID, coopID, cxp)
		// You can also store these values in a data structure if needed
		// For example, you could create a struct to hold this information
		// and append it to a slice for later use.

	}
}

// GetContractArchiveFromAPI will download the events from the Egg Inc API
func GetContractArchiveFromAPI(s *discordgo.Session) {
	userID := config.EIUserIDBasic
	reqURL := "https://www.auxbrain.com/ei_ctx/get_contracts_archive"
	enc := base64.StdEncoding
	clientVersion := uint32(99)

	contractArchiveRequest := BasicRequestInfo{
		EiUserId:      &userID,
		ClientVersion: &clientVersion,
	}
	reqBin, err := proto.Marshal(&contractArchiveRequest)
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

	decodedAuthBuf := &AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return
	}

	contractsArchiveResponse := &ContractsArchive{}
	err = proto.Unmarshal(decodedAuthBuf.Message, contractsArchiveResponse)
	if err != nil {
		log.Print(err)
		return
	}

	archive := contractsArchiveResponse.GetArchive()
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return
	}
	printArchivedContracts(archive)
}
