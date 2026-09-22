package boost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	defineOrderHandlerPrefix = "bo_define"
	defineOrderSessionTTL    = 30 * time.Minute
)

// CustomBoostOrderTemplate represents a named custom boost order definition.
type CustomBoostOrderTemplate struct {
	Name      string   `json:"name"`
	Lines     []string `json:"lines"` // 3 tiebreaker levels
	CreatorID string   `json:"creator_id,omitempty"`
	IsGlobal  bool     `json:"is_global,omitempty"`
}

type defineOrderSession struct {
	uuidStr      string
	userID       string
	channelID    string
	contractHash string
	template     CustomBoostOrderTemplate
	expiresAt    time.Time
}

var (
	defineOrderSessions      = make(map[string]*defineOrderSession)
	defineOrderSessionsMutex sync.Mutex
)

func getOrCreateDefineSession(userID string, contract *Contract, tmpl CustomBoostOrderTemplate) *defineOrderSession {
	defineOrderSessionsMutex.Lock()
	defer defineOrderSessionsMutex.Unlock()

	now := time.Now()
	for k, s := range defineOrderSessions {
		if s.expiresAt.Before(now) {
			delete(defineOrderSessions, k)
		}
	}

	contractHash := ""
	channelID := ""
	if contract != nil {
		contractHash = contract.ContractHash
		if len(contract.Location) > 0 {
			channelID = contract.Location[0].ChannelID
		}
	}

	session := &defineOrderSession{
		uuidStr:      uuid.NewV7().String(),
		userID:       userID,
		channelID:    channelID,
		contractHash: contractHash,
		template:     tmpl,
		expiresAt:    now.Add(defineOrderSessionTTL),
	}
	defineOrderSessions[session.uuidStr] = session
	return session
}

func getDefineSession(uuidStr string) *defineOrderSession {
	defineOrderSessionsMutex.Lock()
	defer defineOrderSessionsMutex.Unlock()
	return defineOrderSessions[uuidStr]
}

func clearDefineSession(uuidStr string) {
	defineOrderSessionsMutex.Lock()
	defer defineOrderSessionsMutex.Unlock()
	delete(defineOrderSessions, uuidStr)
}

// Storage Helpers for User and Global Custom Orders

// GetUserCustomOrders returns all custom boost order templates saved by a user.
func GetUserCustomOrders(userID string) []CustomBoostOrderTemplate {
	raw := farmerstate.GetMiscSettingString(userID, "saved_custom_orders")
	if raw == "" {
		return nil
	}
	var orders []CustomBoostOrderTemplate
	if err := json.Unmarshal([]byte(raw), &orders); err != nil {
		return nil
	}
	return orders
}

// SaveUserCustomOrder saves or updates a custom boost order template for a user.
func SaveUserCustomOrder(userID string, tmpl CustomBoostOrderTemplate) {
	orders := GetUserCustomOrders(userID)
	found := false
	for i, o := range orders {
		if strings.EqualFold(o.Name, tmpl.Name) {
			orders[i] = tmpl
			found = true
			break
		}
	}
	if !found {
		orders = append(orders, tmpl)
	}
	if data, err := json.Marshal(orders); err == nil {
		farmerstate.SetMiscSettingString(userID, "saved_custom_orders", string(data))
	}
}

// GetGlobalCustomOrders returns all globally published custom boost order templates.
func GetGlobalCustomOrders() []CustomBoostOrderTemplate {
	raw := farmerstate.GetMiscSettingString("GLOBAL", "global_custom_orders")
	if raw == "" {
		return nil
	}
	var orders []CustomBoostOrderTemplate
	if err := json.Unmarshal([]byte(raw), &orders); err != nil {
		return nil
	}
	return orders
}

// PublishGlobalCustomOrder saves or updates a custom boost order globally across all servers.
func PublishGlobalCustomOrder(tmpl CustomBoostOrderTemplate) {
	tmpl.IsGlobal = true
	orders := GetGlobalCustomOrders()
	found := false
	for i, o := range orders {
		if strings.EqualFold(o.Name, tmpl.Name) {
			orders[i] = tmpl
			found = true
			break
		}
	}
	if !found {
		orders = append(orders, tmpl)
	}
	if data, err := json.Marshal(orders); err == nil {
		farmerstate.SetMiscSettingString("GLOBAL", "global_custom_orders", string(data))
	}
}

// FindCustomOrderTemplate searches for a template by name across user and global orders.
func FindCustomOrderTemplate(userID string, name string) *CustomBoostOrderTemplate {
	clean := strings.TrimSpace(name)
	if strings.HasPrefix(clean, "user:") {
		clean = strings.TrimPrefix(clean, "user:")
	} else if strings.HasPrefix(clean, "global:") {
		clean = strings.TrimPrefix(clean, "global:")
	}

	for _, o := range GetUserCustomOrders(userID) {
		if strings.EqualFold(o.Name, clean) {
			copyO := o
			return &copyO
		}
	}
	for _, o := range GetGlobalCustomOrders() {
		if strings.EqualFold(o.Name, clean) {
			copyO := o
			return &copyO
		}
	}
	return nil
}

// Slash Command Definition

// GetSlashDefineCustomOrderCommand defines the /define-custom-order slash command.
func GetSlashDefineCustomOrderCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Define, preview, or publish a custom boost order")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "order",
			Description:  "Select an existing custom order to view/modify, or <NEW> to create one",
			Required:     false,
			Autocomplete: true,
		},
	}
	return &command
}

// HandleDefineCustomOrderAutoComplete provides autocomplete options for /define-custom-order.
func HandleDefineCustomOrderAutoComplete(e *dc.AutocompleteEvent) {
	_, focusedVal := e.FocusedOption()
	focused := strings.ToLower(strings.TrimSpace(focusedVal))
	var choices []dc.Choice[string]

	// Always offer <NEW> as the first choice
	if focused == "" || strings.Contains("<new>", focused) || strings.Contains("new", focused) {
		choices = append(choices, dc.Choice[string]{
			Name:  "<NEW> (Create a new custom boost order)",
			Value: "<NEW>",
		})
	}

	// User-saved orders
	userOrders := GetUserCustomOrders(e.UserID())
	for _, o := range userOrders {
		label := fmt.Sprintf("[User] %s", o.Name)
		if focused == "" || strings.Contains(strings.ToLower(label), focused) || strings.Contains(strings.ToLower(o.Name), focused) {
			choices = append(choices, dc.Choice[string]{
				Name:  label,
				Value: "user:" + o.Name,
			})
		}
	}

	// Global orders
	globalOrders := GetGlobalCustomOrders()
	for _, o := range globalOrders {
		label := fmt.Sprintf("[Global] %s", o.Name)
		if focused == "" || strings.Contains(strings.ToLower(label), focused) || strings.Contains(strings.ToLower(o.Name), focused) {
			choices = append(choices, dc.Choice[string]{
				Name:  label,
				Value: "global:" + o.Name,
			})
		}
	}

	if len(choices) > 25 {
		choices = choices[:25]
	}

	_ = e.RespondChoices(choices)
}

// HandleDefineCustomOrderCommand handles invocation of /define-custom-order.
func HandleDefineCustomOrderCommand(_ dc.Client, e *dc.CommandEvent) {
	orderArg, _ := e.OptString("order")
	orderArg = strings.TrimSpace(orderArg)

	if orderArg == "" || orderArg == "<NEW>" {
		SendDefineCustomOrderModalFromCommand(e, nil)
		return
	}

	tmpl := FindCustomOrderTemplate(e.UserID(), orderArg)
	if tmpl == nil {
		// If not found by exact match, open modal prefilling the name typed by user
		SendDefineCustomOrderModalFromCommand(e, &CustomBoostOrderTemplate{Name: orderArg})
		return
	}

	contract := FindContract(e.ChannelID())
	session := getOrCreateDefineSession(e.UserID(), contract, *tmpl)
	msg := BuildDefineCustomOrderMessage(contract, *tmpl, session.uuidStr, "")
	_ = e.Respond(msg)
}

// Modal Presentation & Submission

// SendDefineCustomOrderModalFromCommand presents the 4-line criteria modal in response to a CommandEvent.
func SendDefineCustomOrderModalFromCommand(e *dc.CommandEvent, initial *CustomBoostOrderTemplate) {
	nameVal := "Custom Order"
	lvl1Val := "<DEFL_EFFORT[50]"
	lvl2Val := "<IHR[6%]"
	lvl3Val := ">TOKENS"
	lvl4Val := "<TE"

	if initial != nil {
		if initial.Name != "" {
			nameVal = initial.Name
		}
		if len(initial.Lines) > 0 && initial.Lines[0] != "" {
			lvl1Val = initial.Lines[0]
		}
		if len(initial.Lines) > 1 && initial.Lines[1] != "" {
			lvl2Val = initial.Lines[1]
		} else {
			lvl2Val = ""
		}
		if len(initial.Lines) > 2 && initial.Lines[2] != "" {
			lvl3Val = initial.Lines[2]
		} else {
			lvl3Val = ""
		}
		if len(initial.Lines) > 3 && initial.Lines[3] != "" {
			lvl4Val = initial.Lines[3]
		} else {
			lvl4Val = ""
		}
	}

	session := getOrCreateDefineSession(e.UserID(), FindContract(e.ChannelID()), CustomBoostOrderTemplate{
		Name:  nameVal,
		Lines: []string{lvl1Val, lvl2Val, lvl3Val, lvl4Val},
	})

	_ = e.ShowModal(dc.Modal{
		CustomID: fmt.Sprintf("m_define_order#%s", session.uuidStr),
		Title:    "Custom Boost Order Criteria",
		Inputs: []dc.TextInput{
			{
				CustomID:    "custom-order-level-1",
				Label:       "Level 1 (Primary Condition)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "<DEFL_EFFORT[50]",
				Value:       lvl1Val,
				MaxLength:   100,
				Required:    true,
			},
			{
				CustomID:    "custom-order-level-2",
				Label:       "Level 2 (Tiebreaker 1)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "<IHR[6%]",
				Value:       lvl2Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-3",
				Label:       "Level 3 (Tiebreaker 2)",
				Style:       dc.TextInputStyleShort,
				Placeholder: ">TOKENS",
				Value:       lvl3Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-4",
				Label:       "Level 4 (Tiebreaker 3)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "<TE",
				Value:       lvl4Val,
				MaxLength:   100,
				Required:    false,
			},
		},
	})
}

// SendDefineCustomOrderModalFromComponent presents the 4-line criteria modal in response to a ComponentEvent (e.g. MODIFY button).
func SendDefineCustomOrderModalFromComponent(e *dc.ComponentEvent, tmpl CustomBoostOrderTemplate, sessionUUID string) {
	lvl1Val := ""
	lvl2Val := ""
	lvl3Val := ""
	lvl4Val := ""

	if len(tmpl.Lines) > 0 {
		lvl1Val = tmpl.Lines[0]
	}
	if len(tmpl.Lines) > 1 {
		lvl2Val = tmpl.Lines[1]
	}
	if len(tmpl.Lines) > 2 {
		lvl3Val = tmpl.Lines[2]
	}
	if len(tmpl.Lines) > 3 {
		lvl4Val = tmpl.Lines[3]
	}

	_ = e.ShowModal(dc.Modal{
		CustomID: fmt.Sprintf("m_define_order#%s", sessionUUID),
		Title:    "Custom Boost Order Criteria",
		Inputs: []dc.TextInput{
			{
				CustomID:    "custom-order-level-1",
				Label:       "Level 1 (Primary Condition)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "<DEFL_EFFORT[50]",
				Value:       lvl1Val,
				MaxLength:   100,
				Required:    true,
			},
			{
				CustomID:    "custom-order-level-2",
				Label:       "Level 2 (Tiebreaker 1)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "<IHR[6%]",
				Value:       lvl2Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-3",
				Label:       "Level 3 (Tiebreaker 2)",
				Style:       dc.TextInputStyleShort,
				Placeholder: ">TOKENS",
				Value:       lvl3Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-4",
				Label:       "Level 4 (Tiebreaker 3)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "<TE",
				Value:       lvl4Val,
				MaxLength:   100,
				Required:    false,
			},
		},
	})
}

// HandleDefineCustomOrderModalSubmit processes the submitted 4-line criteria modal.
func HandleDefineCustomOrderModalSubmit(_ dc.Client, e *dc.ModalEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		_ = e.Respond(dc.Message{Content: "Invalid modal submission.", Ephemeral: true})
		return
	}
	sessionUUID := parts[1]

	lvl1 := strings.TrimSpace(e.TextValue("custom-order-level-1"))
	lvl2 := strings.TrimSpace(e.TextValue("custom-order-level-2"))
	lvl3 := strings.TrimSpace(e.TextValue("custom-order-level-3"))
	lvl4 := strings.TrimSpace(e.TextValue("custom-order-level-4"))

	session := getDefineSession(sessionUUID)
	contract := FindContract(e.ChannelID())
	name := "Custom Order"
	if session != nil && session.template.Name != "" {
		name = session.template.Name
	}

	tmpl := CustomBoostOrderTemplate{
		Name:      name,
		Lines:     []string{lvl1, lvl2, lvl3, lvl4},
		CreatorID: e.UserID(),
	}

	if session == nil {
		session = getOrCreateDefineSession(e.UserID(), contract, tmpl)
	} else {
		session.template = tmpl
	}

	msg := BuildDefineCustomOrderMessage(contract, tmpl, session.uuidStr, "✓ Criteria updated! Click **SAVE** or **PUBLISH** to name and save, or **APPLY** to use now.")
	_ = e.Respond(msg)
}

// SendSaveCustomOrderModal presents a modal dialog to name and save the custom order to user presets.
func SendSaveCustomOrderModal(e *dc.ComponentEvent, tmpl CustomBoostOrderTemplate, sessionUUID string) {
	nameVal := tmpl.Name
	if nameVal == "Custom Order" {
		nameVal = ""
	}
	_ = e.ShowModal(dc.Modal{
		CustomID: fmt.Sprintf("m_save_order#%s", sessionUUID),
		Title:    "Save Custom Boost Order",
		Inputs: []dc.TextInput{
			{
				CustomID:    "save-order-name",
				Label:       "Name (What this order is saved as)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "e.g. Deflector Effort & IHR",
				Value:       nameVal,
				MaxLength:   50,
				Required:    true,
			},
		},
	})
}

// HandleSaveCustomOrderModalSubmit processes saving the named custom order preset.
func HandleSaveCustomOrderModalSubmit(_ dc.Client, e *dc.ModalEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		_ = e.Respond(dc.Message{Content: "Invalid save submission.", Ephemeral: true})
		return
	}
	sessionUUID := parts[1]

	session := getDefineSession(sessionUUID)
	if session == nil {
		_ = e.Respond(dc.Message{Content: "This session has expired. Please run `/define-custom-order` again.", Ephemeral: true})
		return
	}

	name := strings.TrimSpace(e.TextValue("save-order-name"))
	if name == "" {
		name = "Custom Order"
	}
	session.template.Name = name
	SaveUserCustomOrder(session.userID, session.template)

	contract := FindContract(e.ChannelID())
	if contract == nil && session.contractHash != "" {
		contract = FindContractByHash(session.contractHash)
	}
	msg := BuildDefineCustomOrderMessage(contract, session.template, sessionUUID, fmt.Sprintf("✅ Saved **%s** to your personal custom boost orders!", name))
	_ = e.Respond(msg)
}

// SendPublishCustomOrderModal presents a modal dialog to name and publish the custom order globally.
func SendPublishCustomOrderModal(e *dc.ComponentEvent, tmpl CustomBoostOrderTemplate, sessionUUID string) {
	nameVal := tmpl.Name
	if nameVal == "Custom Order" {
		nameVal = ""
	}
	_ = e.ShowModal(dc.Modal{
		CustomID: fmt.Sprintf("m_publish_order#%s", sessionUUID),
		Title:    "Publish Global Boost Order",
		Inputs: []dc.TextInput{
			{
				CustomID:    "publish-order-name",
				Label:       "Global Name (Visible across all servers)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "e.g. Deflector Effort & IHR",
				Value:       nameVal,
				MaxLength:   50,
				Required:    true,
			},
		},
	})
}

// HandlePublishCustomOrderModalSubmit processes publishing the named custom order preset globally.
func HandlePublishCustomOrderModalSubmit(_ dc.Client, e *dc.ModalEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 2 {
		_ = e.Respond(dc.Message{Content: "Invalid publish submission.", Ephemeral: true})
		return
	}
	sessionUUID := parts[1]

	session := getDefineSession(sessionUUID)
	if session == nil {
		_ = e.Respond(dc.Message{Content: "This session has expired. Please run `/define-custom-order` again.", Ephemeral: true})
		return
	}

	name := strings.TrimSpace(e.TextValue("publish-order-name"))
	if name == "" {
		name = "Custom Order"
	}
	session.template.Name = name
	PublishGlobalCustomOrder(session.template)

	contract := FindContract(e.ChannelID())
	if contract == nil && session.contractHash != "" {
		contract = FindContractByHash(session.contractHash)
	}
	msg := BuildDefineCustomOrderMessage(contract, session.template, sessionUUID, fmt.Sprintf("🌍 Published **%s** as a Global Boost Order across all servers!", name))
	_ = e.Respond(msg)
}

// Sample Booster Cohort for preview when outside a contract channel
func buildBenchmarkSampleContract() *Contract {
	c := &Contract{
		ContractID:   "benchmark-coop",
		CoopID:       "sample",
		ContractHash: "sample-hash",
		Boosters:     make(map[string]*Booster),
		Order:        []string{"alice", "bob", "charlie", "dana", "eve", "frank"},
	}

	c.Boosters["alice"] = &Booster{
		UserID:       "alice",
		Nick:         "Alice (T4L Owner)",
		IHRRate:      9.2e9,
		TokensWanted: 6,
		TECount:      140,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4L"}},
			LayRate:   24.5,
		},
	}
	c.Boosters["bob"] = &Booster{
		UserID:       "bob",
		Nick:         "Bob (70 Crafts)",
		IHRRate:      10.5e9,
		TokensWanted: 6,
		TECount:      110,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4E"}},
			LayRate:   22.0,
		},
	}
	c.Boosters["charlie"] = &Booster{
		UserID:       "charlie",
		Nick:         "Charlie (50 Crafts)",
		IHRRate:      8.8e9,
		TokensWanted: 6,
		TECount:      95,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T4R"}},
			LayRate:   20.0,
		},
	}
	c.Boosters["dana"] = &Booster{
		UserID:       "dana",
		Nick:         "Dana (42 Crafts)",
		IHRRate:      11.0e9,
		TokensWanted: 6,
		TECount:      80,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T3R"}},
			LayRate:   18.5,
		},
	}
	c.Boosters["eve"] = &Booster{
		UserID:       "eve",
		Nick:         "Eve (20 Crafts)",
		IHRRate:      12.0e9,
		TokensWanted: 4,
		TECount:      75,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "T3R"}},
			LayRate:   17.0,
		},
	}
	c.Boosters["frank"] = &Booster{
		UserID:       "frank",
		Nick:         "Frank (5 Crafts)",
		IHRRate:      7.5e9,
		TokensWanted: 8,
		TECount:      50,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "None"}},
			LayRate:   14.0,
		},
	}
	return c
}

// ⚙️ Preview Message Builder

// BuildDefineCustomOrderMessage creates the ⚙️ report message with table image and action buttons.
func BuildDefineCustomOrderMessage(contract *Contract, tmpl CustomBoostOrderTemplate, sessionUUID string, status string) dc.Message {
	evalContract := contract
	isSample := false
	if evalContract == nil || len(evalContract.Boosters) == 0 {
		evalContract = buildBenchmarkSampleContract()
		isSample = true
	}

	var headerSb strings.Builder
	fmt.Fprintf(&headerSb, "## ⚙️ Custom Boost Order: **%s**\n", tmpl.Name)
	if isSample {
		headerSb.WriteString("-# *Showing preview using sample booster cohort.*\n")
	} else {
		fmt.Fprintf(&headerSb, "**Contract:** `%s` | **Coop:** `%s`\n", evalContract.ContractID, evalContract.CoopID)
	}

	headerSb.WriteString("**Tiebreaker Hierarchy:**\n")
	for i := 0; i < 4; i++ {
		line := "-"
		if i < len(tmpl.Lines) && strings.TrimSpace(tmpl.Lines[i]) != "" {
			line = tmpl.Lines[i]
		}
		fmt.Fprintf(&headerSb, "-# **(%d)** `%s`\n", i+1, line)
	}

	buttons := []dc.InteractiveComponent{
		dc.Button{
			Label:    "MODIFY",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("%s#%s#modify", defineOrderHandlerPrefix, sessionUUID),
		},
		dc.Button{
			Label:    "SAVE",
			Style:    dc.ButtonPrimary,
			CustomID: fmt.Sprintf("%s#%s#save", defineOrderHandlerPrefix, sessionUUID),
		},
		dc.Button{
			Label:    "PUBLISH",
			Style:    dc.ButtonSuccess,
			CustomID: fmt.Sprintf("%s#%s#publish", defineOrderHandlerPrefix, sessionUUID),
		},
	}
	if !isSample && evalContract != nil {
		buttons = append(buttons, dc.Button{
			Label:    "APPLY",
			Style:    dc.ButtonSuccess,
			CustomID: fmt.Sprintf("%s#%s#apply", defineOrderHandlerPrefix, sessionUUID),
		})
	}
	buttons = append(buttons, dc.Button{
		Label:    "DISMISS",
		Style:    dc.ButtonDanger,
		CustomID: fmt.Sprintf("%s#%s#dismiss", defineOrderHandlerPrefix, sessionUUID),
	})

	actionRow := dc.ActionRow{
		Components: buttons,
	}

	imgBytes, err := RenderCustomOrderTableImage(evalContract, tmpl.Lines)

	components := []dc.LayoutComponent{
		dc.TextDisplay{Content: headerSb.String()},
	}

	var files []dc.File
	if err == nil && len(imgBytes) > 0 {
		components = append(components, dc.MediaGallery{
			Items: []dc.MediaItem{{URL: "attachment://custom_order_sort.png"}},
		})
		files = []dc.File{{
			Name:        "custom_order_sort.png",
			ContentType: "image/png",
			Reader:      bytes.NewReader(imgBytes),
		}}
	}

	if status != "" {
		components = append(components, dc.TextDisplay{Content: status})
	}
	components = append(components, actionRow)

	return dc.Message{
		Components: components,
		Files:      files,
		Ephemeral:  true,
	}
}

// HandleDefineCustomOrderReactions handles MODIFY, SAVE, PUBLISH, DISMISS button clicks.
func HandleDefineCustomOrderReactions(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		_ = e.Update(dc.Message{Content: "Invalid button interaction.", Ephemeral: true})
		return
	}

	sessionUUID := parts[1]
	action := parts[2]

	session := getDefineSession(sessionUUID)
	if session == nil {
		_ = e.Update(dc.Message{Content: "This session has expired. Please run `/define-custom-order` again.", Ephemeral: true})
		return
	}

	switch action {
	case "modify":
		SendDefineCustomOrderModalFromComponent(e, session.template, sessionUUID)

	case "save":
		SendSaveCustomOrderModal(e, session.template, sessionUUID)

	case "publish":
		SendPublishCustomOrderModal(e, session.template, sessionUUID)

	case "apply":
		_ = e.DeferUpdate()
		contract := FindContract(e.ChannelID())
		if contract == nil && session.contractHash != "" {
			contract = FindContractByHash(session.contractHash)
		}
		if contract != nil {
			contract.mutex.Lock()
			contract.CustomOrderLines = append([]string(nil), session.template.Lines...)
			contract.BoostOrder = ContractOrderCustom
			unselected := append([]string(nil), contract.Order...)
			contract.Order = sortCustomRemaining(contract, unselected, contract.CustomOrderLines, false)
			contract.mutex.Unlock()

			saveData(contract.ContractHash)
			refreshBoostListMessage(client, contract, false)
			msg := BuildDefineCustomOrderMessage(contract, session.template, sessionUUID, fmt.Sprintf("✅ Applied **%s** to contract `%s`!", session.template.Name, contract.ContractID))
			_ = e.Update(msg)
			return
		}
		_ = e.Update(dc.Message{Content: "Unable to find active contract to apply to.", Ephemeral: true})

	case "dismiss":
		clearDefineSession(sessionUUID)
		_ = e.Update(dc.Message{
			Content:   "Custom Boost Order preview dismissed.",
			Ephemeral: true,
		})

	default:
		_ = e.Update(dc.Message{Content: "Unknown button action.", Ephemeral: true})
	}
}
