package ei

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/wI2L/jsondiff"
	"google.golang.org/protobuf/proto"
)

// DecryptEID decrypts an encrypted Egg Inc ID
func DecryptEID(encryptedString string) string {

	if encryptedString != "" && len(encryptedString) == 18 && encryptedString[:2] == "EI" {
		return encryptedString
	}

	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		return ""
	}
	decodedData, err := base64.StdEncoding.DecodeString(encryptedString)
	if err != nil {
		return ""
	}
	decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
	if err != nil {
		return ""
	}
	return string(decryptedData)
}

// GetFirstContactFromAPI will download the player data from the Egg Inc API
func GetFirstContactFromAPI(s *discordgo.Session, eggIncID string, discordID string, okayToSave bool) (*Backup, bool) {
	eiUserID := DecryptEID(eggIncID)
	reqURL := "https://www.auxbrain.com/ei/bot_first_contact"

	clientVersion := DefaultClientVersion
	platform := DefaultPlatform
	platformString := DefaultPlatformString
	version := DefaultVersion
	build := DefaultBuild

	firstContactRequest := EggIncFirstContactRequest{
		Rinfo: &BasicRequestInfo{
			EiUserId:      &eiUserID,
			ClientVersion: &clientVersion,
			Version:       &version,
			Build:         &build,
			Platform:      &platformString,
		},
		EiUserId:      &eiUserID,
		DeviceId:      proto.String("BoostBot"),
		ClientVersion: &clientVersion,
		Platform:      &platform,
	}

	savefilename := "ttbb-data/eiuserdata/firstcontact-" + discordID + ".pbz"
	protoData, cachedData := APICall(reqURL, &firstContactRequest, okayToSave, 30*time.Second, savefilename)
	if protoData == nil {
		log.Print("GetFirstContactFromAPI: APICall returned nil response")
		return nil, cachedData
	}

	firstContactResponse := &EggIncFirstContactResponse{}
	err := proto.Unmarshal(protoData, firstContactResponse)
	if err != nil {
		log.Print(err)
		return nil, cachedData
	}

	backup := firstContactResponse.GetBackup()
	if backup == nil {
		log.Print("No backup found in Egg Inc API response")
		return nil, cachedData
	}
	// Write the backup as a JSON file for debugging purposes
	if config.IsDevBot() {
		go func() {
			jsonData, err := json.MarshalIndent(backup, "", "  ")
			// Swap all instances of eiUserID with "REDACTED"
			jsonData = []byte(string(jsonData))
			jsonData = []byte(RedactUserInfo(string(jsonData), eiUserID))
			if err != nil {
				log.Println("Error marshalling backup to JSON:", err)
			} else {
				_ = os.MkdirAll("ttbb-data/eiuserdata", os.ModePerm)
				err = os.WriteFile("ttbb-data/eiuserdata/firstcontact-"+discordID+".json", []byte(jsonData), 0644)
				if err != nil {
					log.Print(err)
				}
			}
		}()
	}

	return backup, cachedData
}

// RedactUserInfo will replace all instances of the given eiUserID in the string s with "REDACTED"
func RedactUserInfo(s, eiUserID string) string {
	// redact additional info
	// Delete any lines containing "game_services_id", "push_user_id", or "device_id"
	lines := strings.Split(s, "\n")
	var filteredLines []string
	for _, line := range lines {
		if !strings.Contains(line, "game_services_id") && !strings.Contains(line, "push_user_id") && !strings.Contains(line, "device_id") && !strings.Contains(line, "user_name") {
			filteredLines = append(filteredLines, line)
		}
	}
	s = strings.Join(filteredLines, "\n")
	return strings.ReplaceAll(s, eiUserID, "REDACTED")
}

// GetContractArchiveFromAPI will download the events from the Egg Inc API
func GetContractArchiveFromAPI(s *discordgo.Session, eggIncID string, discordID string, forceRefresh bool, okayToSave bool) ([]*LocalContract, bool) {
	eiUserID := DecryptEID(eggIncID)
	reqURL := "https://www.auxbrain.com/ei_ctx/get_contracts_archive"
	//enc := base64.StdEncoding
	clientVersion := DefaultClientVersion

	contractArchiveRequest := BasicRequestInfo{
		EiUserId:      &eiUserID,
		ClientVersion: &clientVersion,
	}

	savefile := "ttbb-data/eiuserdata/archive-" + discordID + ".pbz"
	cacheDur := 2 * time.Hour
	if forceRefresh {
		cacheDur = 0
	}

	protoData, cachedData := APICall(reqURL, &contractArchiveRequest, okayToSave, cacheDur, savefile)
	if protoData == nil {
		return nil, cachedData
	}

	contractsArchiveResponse := &ContractsArchive{}
	err := proto.Unmarshal(protoData, contractsArchiveResponse)
	if err != nil {
		log.Print(err)
		return nil, cachedData
	}

	archive := contractsArchiveResponse.GetArchive()
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return nil, cachedData
	}

	// Write the archive as a JSON file for debugging purposes
	if config.IsDevBot() {
		go func() {
			jsonData, err := json.MarshalIndent(archive, "", "  ")
			// Redact user info
			jsonData = []byte(RedactUserInfo(string(jsonData), eiUserID))
			if err != nil {
				log.Println("Error marshalling archive to JSON:", err)
			} else {
				_ = os.MkdirAll("ttbb-data/eiuserdata", os.ModePerm)
				err = os.WriteFile("ttbb-data/eiuserdata/archive-"+discordID+".json", []byte(jsonData), 0644)
				if err != nil {
					log.Print(err)
				}
			}
		}()
	}

	return archive, cachedData
}

// GetConfigFromAPI will download the config data from the Egg Inc API and write it to ei-config.json
func GetConfigFromAPI(s *discordgo.Session) bool {
	reqURL := "https://www.auxbrain.com/ei/get_config"

	clientVersion := DefaultClientVersion
	platformString := DefaultPlatformString
	version := DefaultVersion
	build := DefaultBuild

	getConfigRequest := ConfigRequest{
		Rinfo: &BasicRequestInfo{
			EiUserId:      &config.EIUserID,
			ClientVersion: &clientVersion,
			Version:       &version,
			Build:         &build,
			Platform:      &platformString,
		},
	}
	response, _ := APICall(reqURL, &getConfigRequest, false, 0, "")
	if response == nil {
		log.Print("APICall returned nil response")
		return false
	}

	configResponse := &ConfigResponse{}
	opts := proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err := opts.Unmarshal(response, configResponse)
	if err != nil {
		log.Print(err)
		return false
	}

	// Write the config as a JSON file in ttbb-data/ei-config.json for debugging purposes
	go func() {
		jsonData, err := json.MarshalIndent(configResponse, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal config response: %v", err)
			return
		}
		// If the file exists, compare it to the new one to avoid unnecessary writes
		existingData, err := os.ReadFile("ttbb-data/ei-config.json")
		if err == nil {
			if bytes.Equal(existingData, jsonData) {
				// No changes, skip writing
				return
			}
		}
		// Files are different, if we have an existing file, I want a diff
		if len(existingData) > 0 {
			if patch, perr := jsondiff.Compare(existingData, jsonData); perr == nil {
				if b, merr := json.MarshalIndent(patch, "", "    "); merr == nil {
					if strings.Contains(string(b), "ei_hatchery_custom") {
						// If the diff contains the string "ei_hatchery_custom"
						u, err := s.UserChannelCreate(config.AdminUserID)
						if err != nil {
							log.Printf("Failed to create user channel for admin: %v", err)
							return
						}
						var data discordgo.MessageSend
						data.Components = []discordgo.MessageComponent{
							discordgo.TextDisplay{
								Content: fmt.Sprintf("```diff\n%s\n```", string(b)),
							},
						}
						_, sendErr := s.ChannelMessageSendComplex(u.ID, &data)
						if sendErr != nil {
							log.Print(sendErr)
						}
					}
				} else {
					log.Printf("Failed to marshal config diff; proceeding to write file: %v", merr)
				}
			} else {
				log.Printf("Config diff failed; proceeding to write file: %v", perr)
			}
		}

		_ = os.MkdirAll("ttbb-data", os.ModePerm)
		err = os.WriteFile("ttbb-data/ei-config.json", jsonData, 0644)
		if err != nil {
			log.Printf("Failed to write config file: %v", err)
		}
	}()

	return true
}

// loadFromCache attempts to load and decrypt cached payload data
func loadFromCache(savefilename string, cacheDuration time.Duration) ([]byte, bool) {
	if savefilename == "" || cacheDuration <= 0 {
		return nil, false
	}

	fileInfo, err := os.Stat(savefilename)
	if err != nil || time.Since(fileInfo.ModTime()) > cacheDuration {
		return nil, false
	}

	data, err := os.ReadFile(savefilename)
	if err != nil {
		return nil, false
	}

	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		return nil, false
	}

	decryptedData, err := config.DecryptCombined(encryptionKey, data)
	if err != nil {
		return nil, false
	}

	protoData := decryptedData
	if len(decryptedData) >= 2 && decryptedData[0] == 0x1f && decryptedData[1] == 0x8b {
		if gr, zerr := gzip.NewReader(bytes.NewReader(decryptedData)); zerr == nil {
			var buf bytes.Buffer
			if _, zerr = io.Copy(&buf, gr); zerr == nil {
				protoData = buf.Bytes()
			}
			_ = gr.Close()
		}
	}
	return protoData, true
}

// saveToCache compresses, encrypts, and saves the payload in the background
func saveToCache(savefilename string, payloadToSave []byte) {
	if savefilename == "" || len(payloadToSave) == 0 {
		return
	}

	go func() {
		var compressBuf bytes.Buffer
		gw := gzip.NewWriter(&compressBuf)
		if _, err := gw.Write(payloadToSave); err == nil {
			if err = gw.Close(); err == nil {
				encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
				if err == nil {
					combinedData, err := config.EncryptAndCombine(encryptionKey, compressBuf.Bytes())
					if err == nil {
						dir := filepath.Dir(savefilename)
						_ = os.MkdirAll(dir, os.ModePerm)
						err = os.WriteFile(savefilename, combinedData, 0644)
						if err != nil {
							log.Print(err)
						}
					}
				}
			}
		} else {
			_ = gw.Close()
		}
	}()
}

// APICall will make a generic Egg Inc API call with the given request and return the AuthenticatedMessage response
func APICall(reqURL string, request proto.Message, okayToSave bool, cacheDuration time.Duration, savefilename string) ([]byte, bool) {
	if cachedData, ok := loadFromCache(savefilename, cacheDuration); ok {
		return cachedData, true
	}

	enc := base64.StdEncoding

	reqBin, err := proto.Marshal(request)
	if err != nil {
		log.Print(err)
		return nil, false
	}

	values := url.Values{}
	reqDataEncoded := enc.EncodeToString(reqBin)
	values.Set("data", reqDataEncoded)

	response, err := http.PostForm(reqURL, values)
	if err != nil {
		log.Print(err)
		return nil, false
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("Failed to close: %v", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Print(err)
		return nil, false
	}
	protoData := string(body)

	decodedAuthBuf := &AuthenticatedMessage{}
	rawDecodedText, err := enc.DecodeString(protoData)
	if err != nil {
		log.Print(err)
		return nil, false
	}
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		if okayToSave && savefilename != "" {
			saveToCache(savefilename, rawDecodedText)
		}
		return rawDecodedText, false
	}

	// detect if auth_msg.message is compressed
	if decodedAuthBuf.GetCompressed() {
		gr, zerr := zlib.NewReader(bytes.NewReader(decodedAuthBuf.Message))
		if zerr != nil {
			log.Print(zerr)
			return nil, false
		}
		var buf bytes.Buffer
		_, zerr = io.Copy(&buf, gr)
		if zerr != nil {
			log.Print(zerr)
			_ = gr.Close()
			return nil, false
		}
		_ = gr.Close()
		decodedAuthBuf.Message = buf.Bytes()
	}

	if okayToSave {
		saveToCache(savefilename, decodedAuthBuf.Message)
	}

	return decodedAuthBuf.Message, false
}

func getSecureHash(data []byte) (string, error) {
	binaryPath := "./secure_hasher"
	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("secure hasher binary not found at %s", binaryPath)
		}
		return "", fmt.Errorf("failed to stat secure hasher binary: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("secure hasher path is a directory: %s", binaryPath)
	}

	cmd := exec.Command(binaryPath)

	// Pipe the raw bytes into the binary's stdin
	cmd.Stdin = bytes.NewReader(data)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute hasher: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

// APIAuthenticatedCall wraps the request in an AuthenticatedMessage before sending, then decodes the response the same way as APICall.
func APIAuthenticatedCall(reqURL string, request proto.Message) []byte {
	enc := base64.StdEncoding

	innerBin, err := proto.Marshal(request)
	if err != nil {
		log.Print(err)
		return nil
	}

	secureHash, err := getSecureHash(innerBin)
	if err != nil {
		log.Print(err)
		return nil
	}

	authMsg := &AuthenticatedMessage{
		Message: innerBin,
		Code:    &secureHash,
	}

	reqBin, err := proto.Marshal(authMsg)
	if err != nil {
		log.Print(err)
		return nil
	}

	values := url.Values{}
	values.Set("data", enc.EncodeToString(reqBin))

	response, err := http.PostForm(reqURL, values)
	if err != nil {
		log.Print(err)
		return nil
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("Failed to close: %v", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Print(err)
		return nil
	}
	protoData := string(body)

	decodedAuthBuf := &AuthenticatedMessage{}
	rawDecodedText, err := enc.DecodeString(protoData)
	if err != nil {
		log.Print(err)
		return nil
	}
	err = proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return rawDecodedText
	}

	if decodedAuthBuf.GetCompressed() {
		gr, zerr := zlib.NewReader(bytes.NewReader(decodedAuthBuf.Message))
		if zerr != nil {
			log.Print(zerr)
			return nil
		}
		var buf bytes.Buffer
		_, zerr = io.Copy(&buf, gr)
		if zerr != nil {
			log.Print(zerr)
			_ = gr.Close()
			return nil
		}
		_ = gr.Close()
		decodedAuthBuf.Message = buf.Bytes()
	}

	return decodedAuthBuf.Message
}
