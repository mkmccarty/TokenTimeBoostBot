package ei

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"google.golang.org/protobuf/proto"
)

type eiData struct {
	ID                  string
	timestamp           time.Time
	expirationTimestamp time.Time
	contractID          string
	coopID              string
	protoData           string
}

var (
	// Contracts is a map of contracts and is saved to disk
	eiDatas map[string]*eiData
)

func init() {
	eiDatas = make(map[string]*eiData)
}

// GetCoopStatus retrieves the coop status for a given contract and coop
func GetCoopStatus(contractID string, coopID string) (*ContractCoopStatusResponse, time.Time, string, error) {
	eggIncID := config.EIUserIDBasic
	reqURL := "https://www.auxbrain.com/ei/coop_status"
	enc := base64.StdEncoding
	timestamp := time.Now()

	var dataTimestampStr string
	var protoData string

	cacheID := contractID + ":" + coopID
	cachedData := eiDatas[cacheID]

	// Check if the file exists
	if strings.HasPrefix(coopID, "!!") {
		basename := coopID[2:]
		coopID = coopID[2:]
		fname := fmt.Sprintf("ttbb-data/%s-%s.pb", contractID, basename)

		// read the contents of filename into protoData
		fileInfo, err := os.Stat(fname)
		if err != nil {
			return nil, timestamp, dataTimestampStr, err
		}
		fileTimestamp := fileInfo.ModTime()

		protoDataBytes, err := os.ReadFile(fname)
		if err != nil {
			return nil, timestamp, dataTimestampStr, err
		}
		protoData = string(protoDataBytes)

		timestamp = fileTimestamp
		dataTimestampStr = fmt.Sprintf("\nUsing cached data from file %s", fname)
		dataTimestampStr += fmt.Sprintf(", timestamp: %s", fileTimestamp.Format("2006-01-02 15:04:05"))

	} else if cachedData != nil && time.Now().Before(cachedData.expirationTimestamp) {
		protoData = cachedData.protoData
		timestamp = cachedData.timestamp
		dataTimestampStr = fmt.Sprintf("\nUsing cached data retrieved <t:%d:R>, expires <t:%d:R>", cachedData.timestamp.Unix(), cachedData.expirationTimestamp.Unix())
	} else {

		coopStatusRequest := ContractCoopStatusRequest{
			ContractIdentifier: &contractID,
			CoopIdentifier:     &coopID,
			UserId:             &eggIncID,
		}
		timestamp = time.Now()
		reqBin, err := proto.Marshal(&coopStatusRequest)
		if err != nil {
			return nil, timestamp, dataTimestampStr, err
		}

		values := url.Values{}
		reqDataEncoded := enc.EncodeToString(reqBin)
		values.Set("data", string(reqDataEncoded))

		response, err := http.PostForm(reqURL, values)
		if err != nil {
			log.Print(err)
			return nil, timestamp, dataTimestampStr, err
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
			return nil, timestamp, dataTimestampStr, err
		}
		//dataTimestampStr = ""
		protoData = string(body)
		data := eiData{ID: cacheID, timestamp: time.Now(), expirationTimestamp: time.Now().Add(30 * time.Second), contractID: contractID, coopID: coopID, protoData: protoData}
		eiDatas[cacheID] = &data

		// Save protoData into a file
		fileName := fmt.Sprintf("ttbb-data/%s-%s.pb", contractID, coopID)
		err = os.WriteFile(fileName, []byte(protoData), 0644)
		if err != nil {
			log.Print(err)
			return nil, timestamp, dataTimestampStr, err
		}
	}

	decodedAuthBuf := &AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err := proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return nil, timestamp, dataTimestampStr, err
	}

	decodeCoopStatus := &ContractCoopStatusResponse{}
	err = proto.Unmarshal(decodedAuthBuf.Message, decodeCoopStatus)
	if err != nil {
		log.Print(err)
		return nil, timestamp, dataTimestampStr, err
	}

	return decodeCoopStatus, timestamp, dataTimestampStr, nil
}

// ClearCoopStatusCachedData clears the cached data for coop status
func ClearCoopStatusCachedData() {
	var finishHash []string
	for _, d := range eiDatas {
		if d != nil {
			if time.Now().After(d.timestamp) {
				finishHash = append(finishHash, d.ID)
			}
		}
	}
	for _, hash := range finishHash {
		eiDatas[hash] = nil
	}
}
