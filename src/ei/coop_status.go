package ei

import (
	"bytes"
	"compress/gzip"
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
	reqURL := "https://www.auxbrain.com/ei/coop_status_bot"
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
		fname := fmt.Sprintf("ttbb-data/pb/%s-%s.pb", contractID, basename)

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

		// Try to detect if the data is gzip compressed
		if len(protoDataBytes) > 2 && protoDataBytes[0] == 0x1f && protoDataBytes[1] == 0x8b {
			// Data is gzip compressed, decompress it
			gzReader, err := gzip.NewReader(bytes.NewReader(protoDataBytes))
			if err != nil {
				return nil, timestamp, dataTimestampStr, err
			}
			defer func() {
				if err := gzReader.Close(); err != nil {
					log.Printf("Failed to close: %v", err)
				}
			}()

			protoDataBytes, err = io.ReadAll(gzReader)
			if err != nil {
				return nil, timestamp, dataTimestampStr, err
			}
		}
		protoData = string(protoDataBytes)

		timestamp = fileTimestamp
		dataTimestampStr = fmt.Sprintf("\nUsing cached data from file %s", fname)
		dataTimestampStr += fmt.Sprintf("\n%s <t:%d:f>", fileTimestamp.Format("2006-01-02 15:04:05"), fileTimestamp.Unix())

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

		if response.StatusCode != http.StatusOK {
			err := fmt.Errorf("API Error Code: %d for %s/%s", response.StatusCode, contractID, coopID)
			log.Printf("Coop Status Result: %s", err)
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

		if string(body) == "Unauthorized User" {
			err := fmt.Errorf("%s", string(body))
			log.Printf("Coop Status Result: %s", err)
			return nil, timestamp, dataTimestampStr, err
		}

		//dataTimestampStr = ""
		protoData = string(body)
		data := eiData{ID: cacheID, timestamp: time.Now(), expirationTimestamp: time.Now().Add(30 * time.Second), contractID: contractID, coopID: coopID, protoData: protoData}
		eiDatas[cacheID] = &data

		// Save protoData into a file
		fileName := fmt.Sprintf("ttbb-data/pb/%s-%s-%s.pb", contractID, coopID, timestamp.Format("20060102150405"))
		// make sure the directory exists
		if err := os.MkdirAll("ttbb-data/pb", 0755); err != nil {
			log.Print(err)
			return nil, timestamp, dataTimestampStr, err
		}
		var compressedBody bytes.Buffer
		gz := gzip.NewWriter(&compressedBody)
		if _, err = gz.Write([]byte(protoData)); err == nil {
			err = gz.Close()
		}
		if err == nil {
			err = os.WriteFile(fileName, compressedBody.Bytes(), 0644)
		}
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

// GetCoopStatusForCompletedContracts retrieves the coop status for a given contract and coop, but is intended for completed contracts
// This saves the data in compressed form without a timestamp in the filename
func GetCoopStatusForCompletedContracts(contractID string, coopID string) (*ContractCoopStatusResponse, time.Time, string, error) {
	eggIncID := config.EIUserIDBasic
	reqURL := "https://www.auxbrain.com/ei/coop_status_bot"
	enc := base64.StdEncoding
	timestamp := time.Now()

	var dataTimestampStr string
	var protoData string

	// Check to see if this file exists and the timestamp on the file is before expireTime
	fname := fmt.Sprintf("ttbb-data/pb-completed/%s-%s.pb", contractID, coopID)
	// read the contents of filename into protoData
	fileInfo, err := os.Stat(fname)
	if err == nil {
		fileTimestamp := fileInfo.ModTime()

		protoDataBytes, err := os.ReadFile(fname)
		if err != nil {
			return nil, timestamp, dataTimestampStr, err
		}
		// Decompress the data before using
		gzReader, err := gzip.NewReader(bytes.NewReader(protoDataBytes))
		if err != nil {
			return nil, timestamp, dataTimestampStr, err
		}
		decompressedBytes, err := io.ReadAll(gzReader)
		defer func() {
			if err := gzReader.Close(); err != nil {
				// Handle the error appropriately, e.g., logging or taking corrective actions
				log.Printf("Failed to close: %v", err)
			}
		}()

		if err != nil {
			return nil, timestamp, dataTimestampStr, err
		}
		protoData = string(decompressedBytes)

		timestamp = fileTimestamp
		dataTimestampStr = fmt.Sprintf("\nUsing cached data from file %s", fname)
		dataTimestampStr += fmt.Sprintf("\n%s <t:%d:f>", fileTimestamp.Format("2006-01-02 15:04:05"), fileTimestamp.Unix())

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

		protoData = string(body)

		//dataTimestampStr = ""
		// Compress the response body before saving
		var compressedBody bytes.Buffer
		gz := gzip.NewWriter(&compressedBody)
		if _, err := gz.Write(body); err != nil {
			log.Print(err)
			return nil, timestamp, dataTimestampStr, err
		}
		if err := gz.Close(); err != nil {
			log.Print(err)
			return nil, timestamp, dataTimestampStr, err
		}
		protoDataCompressed := compressedBody.String()
		// Save compressedprotoData into a file
		fileName := fmt.Sprintf("ttbb-data/pb-completed/%s-%s.pb", contractID, coopID)
		// make sure the directory exists
		if err := os.MkdirAll("ttbb-data/pb-completed", 0755); err != nil {
			log.Print(err)
			return nil, timestamp, dataTimestampStr, err
		}
		err = os.WriteFile(fileName, []byte(protoDataCompressed), 0644)
		if err != nil {
			log.Print(err)
			return nil, timestamp, dataTimestampStr, err
		}
	}

	decodedAuthBuf := &AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
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
