package boost

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// GetSlashReplayEvalCommand returns the command for the /launch-helper command
func GetSlashReplayEvalCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate contract history and provide replay guidance.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "reset",
				Description: "Reset stored data and start fresh",
				Required:    false,
			},
		},
	}
}

// HandleReplayEval handles the /replay-eval command
func HandleReplayEval(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}

	eggIncID := ""
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(eiID)
		if err == nil {
			decryptedData, err := decryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}

	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "m_replay#" + userID,
				Title:    "BoostBot needs your Egg Inc ID",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "egginc-id",
								Label:       "Egg Inc game ID (El+16 numbers) *",
								Style:       discordgo.TextInputShort,
								Placeholder: "EI0000000000000000",
								MaxLength:   18,
								Required:    true,
							},
						}},
				}}})
		if err != nil {
			log.Println(err.Error())
		}
		return
	}

	// Do the work
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	str := ei.GetContractArchiveFromAPI(s, eggIncID)
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags: flags,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: str},
		},
	})

}

// HandleReplayModalSubmit handles the modal submission for the /replay-eval command
func HandleReplayModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	eggIncID := ""
	str := "That's not a valid Egg Inc ID. It should start with EI followed by 16 numbers."
	userID := bottools.GetInteractionUserID(i)
	modalData := i.ModalSubmitData()
	for _, comp := range modalData.Components {
		input := comp.(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
		if input.CustomID == "egginc-id" && input.Value != "" {
			eggIncID := strings.TrimSpace(input.Value)
			if len(eggIncID) != 18 || eggIncID[:2] != "EI" || !utf8.ValidString(eggIncID) {
				break
			}
			encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
			if err == nil {
				combinedData, err := encryptAndCombine(encryptionKey, []byte(eggIncID))
				if err == nil {
					farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", base64.StdEncoding.EncodeToString(combinedData))
					str = "Egg Inc ID saved."
				}
			}
		}
	}

	flags := discordgo.MessageFlagsIsComponentsV2
	if eggIncID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: str,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Do the work
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	str = ei.GetContractArchiveFromAPI(s, eggIncID)
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags: flags,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: str},
		},
	})
}

/*
// The size of the AES key. 32 bytes for AES-256.
const keySize = 32
// generateKey creates a new, random 32-byte key for AES-256.
func generateKey() ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}
*/

// encryptAndCombine performs AES-GCM encryption and returns the
// nonce and ciphertext combined into a single byte slice.
func encryptAndCombine(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM instance: %w", err)
	}

	// Create a new, unique nonce.
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal the plaintext, which returns the ciphertext.
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	// Prepend the nonce to the ciphertext.
	combined := append(nonce, ciphertext...)

	return combined, nil
}

// decryptCombined performs AES-GCM decryption on a combined byte slice.
// It splits the nonce from the ciphertext and then decrypts the data.
func decryptCombined(key []byte, combined []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM instance: %w", err)
	}

	nonceSize := aesgcm.NonceSize()
	if len(combined) < nonceSize {
		return nil, fmt.Errorf("invalid combined data: too short to contain nonce")
	}

	// Split the combined data into nonce and ciphertext.
	nonce := combined[:nonceSize]
	ciphertext := combined[nonceSize:]

	// Open the data.
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt or authenticate data: %w", err)
	}

	return plaintext, nil
}
