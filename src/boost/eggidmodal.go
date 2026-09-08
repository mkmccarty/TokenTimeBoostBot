package boost

import (
	"encoding/base64"
	"log"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

var (
	optionMapCache      = make(map[string]dc.OptionValues)
	optionMapCacheMutex sync.Mutex
)

// RequestEggIncIDModal sends a modal to the user requesting their Egg Inc ID
func RequestEggIncIDModal(e *dc.CommandEvent, action string, options dc.OptionValues) {
	userID := e.UserID()
	optionMapCacheMutex.Lock()
	optionMapCache[userID] = options
	optionMapCacheMutex.Unlock()

	inputs := []dc.TextInput{
		{
			CustomID:    "egginc-id",
			Label:       "Egg Inc ID (EI+16 digits)",
			Style:       dc.TextInputStyleShort,
			Placeholder: "EI0000000000000000",
			MaxLength:   18,
			Required:    true,
		},
	}

	if action != "register" {
		inputs = append(inputs, dc.TextInput{
			CustomID:    "confirm",
			Label:       "Save or Forget this ID after this session?",
			Style:       dc.TextInputStyleShort,
			Placeholder: "save or forget",
			Value:       "save",
			MaxLength:   6,
			Required:    true,
		})
	}

	title := "BoostBot needs your Egg Inc ID"
	parts := strings.Split(action, "#")
	if parts[0] == "register-alt" && len(parts) > 1 {
		name := parts[1]
		if name == "new" {
			title = "Register New Alternate"
		} else {
			title = "Register Alternate: " + name
		}
	} else if parts[0] == "register" {
		title = "Register Primary Account"
	}

	err := e.ShowModal(dc.Modal{
		CustomID: "m_eggid#" + action,
		Title:    title,
		Inputs:   inputs,
	})
	if err != nil {
		log.Println(err.Error())
	}
}

// HandleEggIDModalSubmit stores the submitted Egg Inc ID and resumes whichever
// command asked for it.
//
// It still takes a raw session because the commands it resumes are not on the
// facade yet.
func HandleEggIDModalSubmit(e *dc.ModalEvent) {
	str := "That's not a valid Egg Inc ID. It should start with EI followed by 16 numbers."
	encryptedID := ""
	okayToSave := false
	targetAlt := "new"
	userID := e.UserID()

	eggIncID := strings.TrimSpace(e.TextValue("egginc-id"))
	if len(eggIncID) == 18 && eggIncID[:2] == "EI" && utf8.ValidString(eggIncID) {
		encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
		if err == nil {
			combinedData, err := config.EncryptAndCombine(encryptionKey, []byte(eggIncID))
			if err == nil {
				encryptedID = base64.StdEncoding.EncodeToString(combinedData)
				str = "Egg Inc ID saved.\nRerun the command to evaluate your contract history."
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(e.TextValue("confirm")), "save") {
		okayToSave = true
	}

	parts := strings.Split(e.CustomID(), "#")
	if parts[1] == "register" || parts[1] == "register-alt" {
		okayToSave = true
	} else {
		if !okayToSave {
			str += "\nI will forget your Egg Inc ID after this session."
		}
	}

	if okayToSave && encryptedID != "" {
		if parts[1] != "register-alt" {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", encryptedID)
		}
		str += "\nI will remember your Egg Inc ID for future sessions."
	}

	if parts[1] == "register-alt" && len(parts) > 2 {
		targetAlt = parts[2]
	}

	optionMapCacheMutex.Lock()
	options := optionMapCache[userID]
	delete(optionMapCache, userID)
	optionMapCacheMutex.Unlock()

	switch parts[1] {
	case "register":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to register."
			break
		}
		Register(e, encryptedID, okayToSave)
		return
	case "register-alt":
		RegisterAlt(e, targetAlt, encryptedID)
		return
	case "replay":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		RerunEval(e, options, encryptedID, okayToSave)
		return
	case "virtue":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		Virtue(e, options, encryptedID, okayToSave)
		return
	case "contract-report":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		err := ContractReport(e, options, encryptedID, okayToSave)
		// This should not happen, but just in case
		if err != nil {
			log.Println("Error in ContractReport after EggID modal:", err)
		}
		return
	default:
	}

	_ = e.Respond(dc.Message{Content: str, Ephemeral: true})
}
