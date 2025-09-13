package ei

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
	"unicode/utf8"

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
	printArchivedContracts(archive, 20)
}

// GetContractArchiveFromAPI will download the events from the Egg Inc API
func GetContractArchiveFromAPI(s *discordgo.Session, eiUserID string) string {
	reqURL := "https://www.auxbrain.com/ei_ctx/get_contracts_archive"
	enc := base64.StdEncoding
	clientVersion := uint32(99)

	contractArchiveRequest := BasicRequestInfo{
		EiUserId:      &eiUserID,
		ClientVersion: &clientVersion,
	}
	reqBin, err := proto.Marshal(&contractArchiveRequest)
	if err != nil {
		log.Print(err)
		return ""
	}
	values := url.Values{}
	reqDataEncoded := enc.EncodeToString(reqBin)
	values.Set("data", string(reqDataEncoded))

	response, err := http.PostForm(reqURL, values)
	if err != nil {
		log.Print(err)
		return ""
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
		return ""
	}

	protoData := string(body)

	decodedAuthBuf := &AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return ""
	}

	contractsArchiveResponse := &ContractsArchive{}
	err = proto.Unmarshal(decodedAuthBuf.Message, contractsArchiveResponse)
	if err != nil {
		log.Print(err)
		return ""
	}

	archive := contractsArchiveResponse.GetArchive()
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return ""
	}
	return printArchivedContracts(archive, 20)
}

func printArchivedContracts(archive []*LocalContract, percent int) string {
	builder := strings.Builder{}
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return builder.String()
	}
	log.Printf("Downloaded %d archived contracts from Egg Inc API\n", len(archive))

	// Want a preamble string for builder for what we're displaying
	//builder.WriteString(fmt.Sprintf("## Displaying contract scores less than %d%% of speedrun potential:\n", percent))
	builder.WriteString("## Contract CS eval of active contracts\n")
	fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
		AlignString("CONTRACT-ID", 30, StringAlignCenter),
		AlignString("CS", 6, StringAlignCenter),
		AlignString("HIGH", 6, StringAlignCenter),
		AlignString("GAP", 6, StringAlignRight),
		AlignString("%", 4, StringAlignCenter),
	)

	for _, c := range archive {
		contractID := c.GetContract().GetIdentifier()
		//coopID := c.GetCoopIdentifier()
		evaluation := c.GetEvaluation()
		cxp := evaluation.GetCxp()

		c := EggIncContractsAll[contractID]
		if c.ContractVersion == 2 && c.ExpirationTime.Unix() > time.Now().Unix() {
			// Two ranges of estimates
			// Speedrun w/ perfect set through to Sink with decent set
			// Fair share from 1.05 to 0.92

			//if cxp < c.Cxp*(1-float64(percent)/100) {

			fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s` <t:%d:R>\n",
				AlignString(contractID, 30, StringAlignLeft),
				AlignString(fmt.Sprintf("%d", int(math.Ceil(cxp))), 6, StringAlignRight),
				AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp))), 6, StringAlignRight),
				AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp-cxp))), 6, StringAlignRight),
				AlignString(fmt.Sprintf("%.1f", (cxp/c.Cxp)*100), 4, StringAlignCenter),
				c.ExpirationTime.Unix())
			//}
		}

	}
	return builder.String()
}

// StringAlign is an enum for string alignment
type StringAlign int

const (
	// StringAlignLeft aligns the string to the left
	StringAlignLeft StringAlign = iota
	// StringAlignCenter aligns the string to the center
	StringAlignCenter
	// StringAlignRight aligns the string to the right
	StringAlignRight
)

// AlignString aligns a string to the left, center, or right within a given width
func AlignString(str string, width int, alignment StringAlign) string {
	// Calculate the padding needed

	padding := width - utf8.RuneCountInString(str)
	if padding <= 0 {
		return str
	}

	var leftPadding, rightPadding string
	switch alignment {
	case StringAlignLeft:
		leftPadding = ""
		rightPadding = strings.Repeat(" ", padding)
	case StringAlignCenter:
		leftPadding = strings.Repeat(" ", padding/2)
		rightPadding = strings.Repeat(" ", padding-padding/2)
	case StringAlignRight:
		leftPadding = strings.Repeat(" ", padding)
		rightPadding = ""
	}

	return leftPadding + str + rightPadding
}
