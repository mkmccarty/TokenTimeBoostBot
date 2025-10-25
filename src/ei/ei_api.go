package ei

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"google.golang.org/protobuf/proto"
)

// GetFirstContactFromAPI will download the player data from the Egg Inc API
func GetFirstContactFromAPI(s *discordgo.Session, eiUserID string, discordID string, okayToSave bool) (*Backup, bool) {
	reqURL := "https://www.auxbrain.com//ei/bot_first_contact"
	enc := base64.StdEncoding
	cachedData := false

	protoData := ""
	if fileInfo, err := os.Stat("ttbb-data/eiuserdata/firstcontact-" + discordID + ".pbz"); err == nil {
		// File exists, check if it's within 30 seconds
		if time.Since(fileInfo.ModTime()) <= 30*time.Second {
			// File is recent, load it
			data, err := os.ReadFile("ttbb-data/eiuserdata/firstcontact-" + discordID + ".pbz")
			if err == nil {
				encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
				if err == nil {
					decryptedData, err := config.DecryptCombined(encryptionKey, data)
					if err == nil {
						data = decryptedData
						if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
							if gr, zerr := gzip.NewReader(bytes.NewReader(data)); zerr == nil {
								var buf bytes.Buffer
								if _, zerr = io.Copy(&buf, gr); zerr == nil {
									data = buf.Bytes()
								}
								_ = gr.Close()
							}
						}
						protoData = string(data)
						cachedData = true
						// Successfully decrypted, use protoData
					}
				}
			}
		}
	}

	if protoData == "" {
		clientVersion := uint32(99)

		platform := Platform_IOS
		platformString := "IOS"
		version := "1.35.2"
		build := "111300"

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

		reqBin, err := proto.Marshal(&firstContactRequest)
		if err != nil {
			log.Print(err)
			return nil, cachedData
		}
		values := url.Values{}
		reqDataEncoded := enc.EncodeToString(reqBin)
		values.Set("data", string(reqDataEncoded))

		response, err := http.PostForm(reqURL, values)
		if err != nil {
			log.Print(err)
			return nil, cachedData
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
			return nil, cachedData
		}
		protoData = string(body)

		if okayToSave {
			go func() {
				// Compress protoData first
				var compressBuf bytes.Buffer
				gw := gzip.NewWriter(&compressBuf)
				if _, err := gw.Write([]byte(protoData)); err == nil {
					if err = gw.Close(); err == nil {
						// Then encrypt the compressed data
						encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
						if err == nil {
							combinedData, err := config.EncryptAndCombine(encryptionKey, compressBuf.Bytes())
							if err == nil {
								_ = os.MkdirAll("ttbb-data/eiuserdata", os.ModePerm)
								err = os.WriteFile("ttbb-data/eiuserdata/firstcontact-"+discordID+".pbz", combinedData, 0644)
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
	}

	rawDecodedText, _ := enc.DecodeString(protoData)
	firstContactResponse := &EggIncFirstContactResponse{}
	err := proto.Unmarshal(rawDecodedText, firstContactResponse)
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
func GetContractArchiveFromAPI(s *discordgo.Session, eiUserID string, discordID string, forceRefresh bool, okayToSave bool) ([]*LocalContract, bool) {
	reqURL := "https://www.auxbrain.com/ei_ctx/get_contracts_archive"
	enc := base64.StdEncoding
	clientVersion := uint32(99)
	protoData := ""
	cachedData := false

	if fileInfo, err := os.Stat("ttbb-data/eiuserdata/archive-" + discordID + ".pbz"); err == nil {
		// File exists, check if it's within 1 hour
		if !forceRefresh && time.Since(fileInfo.ModTime()) <= 1*time.Hour {
			// File is recent, load it
			data, err := os.ReadFile("ttbb-data/eiuserdata/archive-" + discordID + ".pbz")
			if err == nil {
				encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
				if err == nil {
					decryptedData, err := config.DecryptCombined(encryptionKey, data)
					if err == nil {
						protoData = string(decryptedData)
						cachedData = true
						// Check if the data is compressed
						if len(decryptedData) >= 2 && decryptedData[0] == 0x1f && decryptedData[1] == 0x8b {
							if gr, zerr := gzip.NewReader(bytes.NewReader(decryptedData)); zerr == nil {
								var buf bytes.Buffer
								if _, zerr = io.Copy(&buf, gr); zerr == nil {
									protoData = buf.String()
								}
								_ = gr.Close()
							}
						}
					}
				}
			}
		}
	}

	if protoData == "" {

		contractArchiveRequest := BasicRequestInfo{
			EiUserId:      &eiUserID,
			ClientVersion: &clientVersion,
		}
		reqBin, err := proto.Marshal(&contractArchiveRequest)
		if err != nil {
			log.Print(err)
			return nil, cachedData
		}
		values := url.Values{}
		reqDataEncoded := enc.EncodeToString(reqBin)
		values.Set("data", string(reqDataEncoded))

		response, err := http.PostForm(reqURL, values)
		if err != nil {
			log.Print(err)
			return nil, cachedData
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
			return nil, cachedData
		}

		protoData = string(body)

		// Encrypt this to save to disk
		if okayToSave {
			go func() {
				// Compress protoData first
				var compressBuf bytes.Buffer
				gw := gzip.NewWriter(&compressBuf)
				if _, err := gw.Write([]byte(protoData)); err == nil {
					if err = gw.Close(); err == nil {
						// Then encrypt the compressed data
						encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
						if err == nil {
							combinedData, err := config.EncryptAndCombine(encryptionKey, compressBuf.Bytes())
							if err == nil {
								_ = os.MkdirAll("ttbb-data/eiuserdata", os.ModePerm)
								err = os.WriteFile("ttbb-data/eiuserdata/archive-"+discordID+".pbz", combinedData, 0644)
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
	}

	decodedAuthBuf := &AuthenticatedMessage{}
	rawDecodedText, _ := enc.DecodeString(protoData)
	err := proto.Unmarshal(rawDecodedText, decodedAuthBuf)
	if err != nil {
		log.Print(err)
		return nil, cachedData
	}

	contractsArchiveResponse := &ContractsArchive{}
	err = proto.Unmarshal(decodedAuthBuf.Message, contractsArchiveResponse)
	if err != nil {
		log.Print(err)
		return nil, cachedData
	}

	archive := contractsArchiveResponse.GetArchive()
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return nil, cachedData
	}
	return archive, cachedData
}
