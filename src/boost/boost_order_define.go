package boost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"uuid"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	defineOrderHandlerPrefix = "bo_define"
	defineOrderSessionTTL    = 30 * time.Minute
	// maxCustomOrderLineLen caps the length of a single criteria line after sanitization.
	maxCustomOrderLineLen = 100
	// maxCustomOrderNameLen caps the length of a saved order name after sanitization.
	maxCustomOrderNameLen = 50
)

// reDiscordMention matches Discord mention patterns that could ping users,
// roles, channels, or use @everyone/@here.
var reDiscordMention = regexp.MustCompile(`<@[!&]?\d+>|<#\d+>`)

// reMarkdownLink matches markdown-style links [text](url) used for phishing.
var reMarkdownLink = regexp.MustCompile(`\[([^\]]{0,100})\]\([^)]*\)`)

// sanitizeCustomOrderInput strips dangerous content from a single criteria
// line entered via the modal. It prevents Discord mention injection,
// markdown link abuse, control characters, and excessive length.
func sanitizeCustomOrderInput(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Remove control characters (except normal space) that could break
	// formatting or hide content.
	s = strings.Map(func(r rune) rune {
		if r == ' ' {
			return r
		}
		if unicode.IsControl(r) {
			return -1 // drop
		}
		// Drop zero-width characters used to hide content
		if r == '\u200B' || r == '\u200C' || r == '\u200D' || r == '\uFEFF' {
			return -1
		}
		return r
	}, s)

	// Strip Discord mention syntax: <@id>, <@!id>, <@&id>, <#id>
	s = reDiscordMention.ReplaceAllString(s, "")

	// Neutralise @everyone and @here (replace @ with fullwidth @)
	s = strings.ReplaceAll(s, "@everyone", "＠everyone")
	s = strings.ReplaceAll(s, "@here", "＠here")
	// Also catch case-insensitive variants
	for _, mention := range []string{"@EVERYONE", "@Everyone", "@HERE", "@Here"} {
		s = strings.ReplaceAll(s, mention, "＠"+mention[1:])
	}

	// Strip markdown links [text](url) to prevent phishing; keep the text part
	s = reMarkdownLink.ReplaceAllString(s, "$1")

	// Enforce max length
	if len(s) > maxCustomOrderLineLen {
		s = s[:maxCustomOrderLineLen]
	}

	return strings.TrimSpace(s)
}

// sanitizeCustomOrderName sanitizes a saved/published order name.
func sanitizeCustomOrderName(s string) string {
	s = sanitizeCustomOrderInput(s)
	if len(s) > maxCustomOrderNameLen {
		s = s[:maxCustomOrderNameLen]
	}
	return strings.TrimSpace(s)
}

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
	isSaved      bool
	expiresAt    time.Time
}

var (
	defineOrderSessions      = make(map[string]*defineOrderSession)
	defineOrderSessionsMutex sync.Mutex
)

func getOrCreateDefineSession(userID string, contract *Contract, tmpl CustomBoostOrderTemplate, isSaved bool) *defineOrderSession {
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
		isSaved:      isSaved,
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

// DeleteUserCustomOrder deletes a custom boost order template for a user by name. Returns true if found and removed.
func DeleteUserCustomOrder(userID string, name string) bool {
	clean := strings.TrimSpace(name)
	clean = strings.TrimPrefix(clean, "user:")
	orders := GetUserCustomOrders(userID)
	newOrders := make([]CustomBoostOrderTemplate, 0, len(orders))
	found := false
	for _, o := range orders {
		if strings.EqualFold(o.Name, clean) {
			found = true
			continue
		}
		newOrders = append(newOrders, o)
	}
	if found {
		if len(newOrders) == 0 {
			farmerstate.SetMiscSettingString(userID, "saved_custom_orders", "")
		} else if data, err := json.Marshal(newOrders); err == nil {
			farmerstate.SetMiscSettingString(userID, "saved_custom_orders", string(data))
		}
	}
	return found
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

// DeleteGlobalCustomOrder deletes a globally published custom boost order template by name. Returns true if found and removed.
func DeleteGlobalCustomOrder(name string) bool {
	clean := strings.TrimSpace(name)
	clean = strings.TrimPrefix(clean, "global:")
	orders := GetGlobalCustomOrders()
	newOrders := make([]CustomBoostOrderTemplate, 0, len(orders))
	found := false
	for _, o := range orders {
		if strings.EqualFold(o.Name, clean) {
			found = true
			continue
		}
		newOrders = append(newOrders, o)
	}
	if found {
		if len(newOrders) == 0 {
			farmerstate.SetMiscSettingString("GLOBAL", "global_custom_orders", "")
		} else if data, err := json.Marshal(newOrders); err == nil {
			farmerstate.SetMiscSettingString("GLOBAL", "global_custom_orders", string(data))
		}
	}
	return found
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

var customBoostOrderDocBytes []byte

// SetCustomBoostOrderDoc stores the documentation content bytes for CustomBoostOrder.md.
func SetCustomBoostOrderDoc(data []byte) {
	customBoostOrderDocBytes = data
}

// GetCustomBoostOrderDoc retrieves the markdown documentation bytes for CustomBoostOrder.md.
func GetCustomBoostOrderDoc() []byte {
	return customBoostOrderDocBytes
}

// Slash Command Definition

// GetSlashCustomBoostOrderCommand defines the /custom-boost-order slash command with craft, delete, and help subcommands.
func GetSlashCustomBoostOrderCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Craft, preview, delete, or view documentation for custom boost orders")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "craft",
			Description: "Craft, define, or preview a custom boost order",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "order",
					Description:  "Select an existing custom order to view/craft, or <NEW> to create one",
					Required:     false,
					Autocomplete: true,
				},
			},
		},
		dc.SubCommand{
			Name:        "delete",
			Description: "Delete a saved custom boost order",
			Options: []dc.Option{
				dc.StringOption{
					Name:         "order",
					Description:  "Select a saved custom order to delete",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		dc.SubCommand{
			Name:        "help",
			Description: "Show documentation and guide for custom boost orders",
		},
	}
	return &command
}

// GetSlashDefineCustomOrderCommand is a backwards-compatible alias for GetSlashCustomBoostOrderCommand.
func GetSlashDefineCustomOrderCommand(cmd string) *dc.Command {
	return GetSlashCustomBoostOrderCommand(cmd)
}

// HandleCustomBoostOrderAutoComplete provides autocomplete options for /custom-boost-order.
func HandleCustomBoostOrderAutoComplete(e *dc.AutocompleteEvent) {
	sub, _ := e.Subcommand()
	if sub != "craft" && sub != "modify" && sub != "delete" {
		_ = e.RespondChoices(nil)
		return
	}

	if sub == "craft" || sub == "modify" {
		if contract := FindContract(e.ChannelID()); contract == nil {
			_ = e.RespondChoices(nil)
			return
		}
	}

	_, focusedVal := e.FocusedOption()
	focused := strings.ToLower(strings.TrimSpace(focusedVal))
	var choices []dc.Choice[string]

	// Only offer <NEW> for craft / modify
	if sub == "craft" || sub == "modify" {
		if focused == "" || strings.Contains("<new>", focused) || strings.Contains("new", focused) {
			choices = append(choices, dc.Choice[string]{
				Name:  "<NEW> (Create a new custom boost order)",
				Value: "<NEW>",
			})
		}
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

// HandleDefineCustomOrderAutoComplete provides backwards-compatible autocomplete.
func HandleDefineCustomOrderAutoComplete(e *dc.AutocompleteEvent) {
	HandleCustomBoostOrderAutoComplete(e)
}

// HandleCustomBoostOrderCommand handles invocation of /custom-boost-order.
func HandleCustomBoostOrderCommand(client dc.Client, e *dc.CommandEvent) {
	subcmd, _ := e.Subcommand()
	switch subcmd {
	case "help":
		data := GetCustomBoostOrderDoc()
		if len(data) == 0 {
			_ = e.Respond(dc.Message{
				Content:   "Custom Boost Order documentation is currently unavailable.",
				Ephemeral: true,
			})
			return
		}
		_ = e.Respond(dc.Message{
			Content:   "📖 **Custom Boost Order Documentation**\nSee the attached `CustomBoostOrder.md` for complete syntax, formulas, artifact criteria, and examples.",
			Ephemeral: true,
			Files: []dc.File{
				{
					Name:        "CustomBoostOrder.md",
					ContentType: "text/markdown",
					Reader:      bytes.NewReader(data),
				},
			},
		})
	case "delete":
		handleCustomBoostOrderDelete(client, e)
	case "craft", "modify", "":
		contract := FindContract(e.ChannelID())
		if contract == nil {
			_ = e.Respond(dc.Message{
				Content:   "This command must be run in a contract channel.",
				Ephemeral: true,
			})
			return
		}
		handleCustomBoostOrderCraft(client, e, contract)
	default:
		_ = e.Respond(dc.Message{Content: fmt.Sprintf("Unknown subcommand %q.", subcmd), Ephemeral: true})
	}
}

// HandleDefineCustomOrderCommand provides backwards-compatible invocation.
func HandleDefineCustomOrderCommand(client dc.Client, e *dc.CommandEvent) {
	HandleCustomBoostOrderCommand(client, e)
}

func handleCustomBoostOrderDelete(_ dc.Client, e *dc.CommandEvent) {
	orderArg, _ := e.OptString("order")
	orderArg = strings.TrimSpace(orderArg)

	if orderArg == "" {
		_ = e.Respond(dc.Message{
			Content:   "Please select a saved custom boost order to delete.",
			Ephemeral: true,
		})
		return
	}

	tmpl := FindCustomOrderTemplate(e.UserID(), orderArg)
	if tmpl == nil {
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("Custom boost order %q not found.", orderArg),
			Ephemeral: true,
		})
		return
	}

	contract := FindContract(e.ChannelID())
	session := getOrCreateDefineSession(e.UserID(), contract, *tmpl, true)

	var sb strings.Builder
	fmt.Fprintf(&sb, "## 🗑️ Delete Custom Boost Order: **%s**\n", tmpl.Name)
	sb.WriteString("Are you sure you want to delete this custom boost order?\n\n")
	sb.WriteString("**Tiebreaker Hierarchy:**\n")
	for i := 0; i < 4; i++ {
		line := "-"
		if i < len(tmpl.Lines) && strings.TrimSpace(tmpl.Lines[i]) != "" {
			line = tmpl.Lines[i]
		}
		fmt.Fprintf(&sb, "-# **(%d)** `%s`\n", i+1, line)
	}

	buttons := []dc.InteractiveComponent{
		dc.Button{
			Label:    "DISMISS",
			Style:    dc.ButtonSecondary,
			CustomID: fmt.Sprintf("%s#%s#del_dismiss", defineOrderHandlerPrefix, session.uuidStr),
		},
		dc.Button{
			Label:    "DELETE",
			Style:    dc.ButtonDanger,
			CustomID: fmt.Sprintf("%s#%s#del_confirm", defineOrderHandlerPrefix, session.uuidStr),
		},
	}

	_ = e.Respond(dc.Message{
		Components: []dc.LayoutComponent{
			dc.TextDisplay{Content: sb.String()},
			dc.ActionRow{Components: buttons},
		},
		Ephemeral: true,
	})
}

func handleCustomBoostOrderCraft(_ dc.Client, e *dc.CommandEvent, contract *Contract) {
	orderArg, _ := e.OptString("order")
	orderArg = strings.TrimSpace(orderArg)

	if orderArg == "" || orderArg == "<NEW>" {
		SendDefineCustomOrderModalFromCommand(e, contract, nil)
		return
	}

	tmpl := FindCustomOrderTemplate(e.UserID(), orderArg)
	if tmpl == nil {
		// If not found by exact match, open modal prefilling the name typed by user
		SendDefineCustomOrderModalFromCommand(e, contract, &CustomBoostOrderTemplate{Name: orderArg})
		return
	}

	session := getOrCreateDefineSession(e.UserID(), contract, *tmpl, true)
	msg := BuildDefineCustomOrderMessage(contract, *tmpl, session.uuidStr, "", true)
	_ = e.Respond(msg)
}

// Modal Presentation & Submission

// SendDefineCustomOrderModalFromCommand presents the 4-line criteria modal in response to a CommandEvent.
func SendDefineCustomOrderModalFromCommand(e *dc.CommandEvent, contract *Contract, initial *CustomBoostOrderTemplate) {
	nameVal := ""
	lvl1Val := ""
	lvl2Val := ""
	lvl3Val := ""
	lvl4Val := ""

	if initial != nil {
		if initial.Name != "" {
			nameVal = initial.Name
		}
		if len(initial.Lines) > 0 && initial.Lines[0] != "" {
			lvl1Val = initial.Lines[0]
		}
		if len(initial.Lines) > 1 && initial.Lines[1] != "" {
			lvl2Val = initial.Lines[1]
		}
		if len(initial.Lines) > 2 && initial.Lines[2] != "" {
			lvl3Val = initial.Lines[2]
		}
		if len(initial.Lines) > 3 && initial.Lines[3] != "" {
			lvl4Val = initial.Lines[3]
		}
	}

	session := getOrCreateDefineSession(e.UserID(), contract, CustomBoostOrderTemplate{
		Name:  nameVal,
		Lines: []string{lvl1Val, lvl2Val, lvl3Val, lvl4Val},
	}, false)

	_ = e.ShowModal(dc.Modal{
		CustomID: fmt.Sprintf("m_define_order#%s", session.uuidStr),
		Title:    "Custom Boost Order Criteria",
		Inputs: []dc.TextInput{
			{
				CustomID:    "custom-order-level-1",
				Label:       "Level 1 (Primary Condition)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "<IHR[6%]",
				Value:       lvl1Val,
				MaxLength:   100,
				Required:    true,
			},
			{
				CustomID:    "custom-order-level-2",
				Label:       "Level 2 (Tiebreaker 1)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "ELR",
				Value:       lvl2Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-3",
				Label:       "Level 3 (Tiebreaker 2)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "",
				Value:       lvl3Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-4",
				Label:       "Level 4 (Tiebreaker 3)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "",
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
				Placeholder: "<IHR[6%]",
				Value:       lvl1Val,
				MaxLength:   100,
				Required:    true,
			},
			{
				CustomID:    "custom-order-level-2",
				Label:       "Level 2 (Tiebreaker 1)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "ELR",
				Value:       lvl2Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-3",
				Label:       "Level 3 (Tiebreaker 2)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "",
				Value:       lvl3Val,
				MaxLength:   100,
				Required:    false,
			},
			{
				CustomID:    "custom-order-level-4",
				Label:       "Level 4 (Tiebreaker 3)",
				Style:       dc.TextInputStyleShort,
				Placeholder: "",
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
	if e.FromComponent() {
		_ = e.DeferUpdate()
	} else {
		_ = e.Defer(true)
	}
	sessionUUID := parts[1]

	lvl1 := sanitizeCustomOrderInput(e.TextValue("custom-order-level-1"))
	lvl2 := sanitizeCustomOrderInput(e.TextValue("custom-order-level-2"))
	lvl3 := sanitizeCustomOrderInput(e.TextValue("custom-order-level-3"))
	lvl4 := sanitizeCustomOrderInput(e.TextValue("custom-order-level-4"))

	session := getDefineSession(sessionUUID)
	contract := FindContract(e.ChannelID())
	if contract == nil && session != nil && session.contractHash != "" {
		contract = FindContractByHash(session.contractHash)
	}
	if contract == nil {
		_ = e.EditResponse(dc.Message{
			Content:   "This command must be run in a contract channel.",
			Ephemeral: true,
		})
		return
	}
	name := ""
	if session != nil && session.template.Name != "" {
		name = session.template.Name
	}

	tmpl := CustomBoostOrderTemplate{
		Name:      name,
		Lines:     []string{lvl1, lvl2, lvl3, lvl4},
		CreatorID: e.UserID(),
	}

	if session == nil {
		session = getOrCreateDefineSession(e.UserID(), contract, tmpl, false)
	} else {
		session.template = tmpl
		session.isSaved = false
	}

	msg := BuildDefineCustomOrderMessage(contract, tmpl, session.uuidStr, "✓ Criteria updated! Click **SAVE** to name and save, then **SELECT** to use.", false)
	_ = e.EditResponse(msg)
}

// SendSaveCustomOrderModal presents a modal dialog to name and save the custom order to user presets.
func SendSaveCustomOrderModal(e *dc.ComponentEvent, tmpl CustomBoostOrderTemplate, sessionUUID string) {
	nameVal := tmpl.Name
	if strings.TrimSpace(nameVal) == "" || strings.EqualFold(nameVal, "Custom Order") {
		nameVal = SuggestCustomOrderName(tmpl.Lines)
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
	if e.FromComponent() {
		_ = e.DeferUpdate()
	} else {
		_ = e.Defer(true)
	}
	sessionUUID := parts[1]

	session := getDefineSession(sessionUUID)
	if session == nil {
		_ = e.EditResponse(dc.Message{Content: "This session has expired. Please run `/custom-boost-order craft` again.", Ephemeral: true})
		return
	}

	name := sanitizeCustomOrderName(e.TextValue("save-order-name"))
	if name == "" {
		name = SuggestCustomOrderName(session.template.Lines)
		if name == "" {
			name = "Custom Order"
		}
	}
	session.template.Name = name
	session.isSaved = true
	SaveUserCustomOrder(session.userID, session.template)

	contract := FindContract(e.ChannelID())
	if contract == nil && session.contractHash != "" {
		contract = FindContractByHash(session.contractHash)
	}
	msg := BuildDefineCustomOrderMessage(contract, session.template, sessionUUID, fmt.Sprintf("✅ Saved **%s** to your personal custom boost orders!", name), true)
	_ = e.EditResponse(msg)
}

// SendPublishCustomOrderModal presents a modal dialog to name and publish the custom order globally.
func SendPublishCustomOrderModal(e *dc.ComponentEvent, tmpl CustomBoostOrderTemplate, sessionUUID string) {
	nameVal := tmpl.Name
	if strings.TrimSpace(nameVal) == "" || strings.EqualFold(nameVal, "Custom Order") {
		nameVal = SuggestCustomOrderName(tmpl.Lines)
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
	if e.FromComponent() {
		_ = e.DeferUpdate()
	} else {
		_ = e.Defer(true)
	}
	sessionUUID := parts[1]

	session := getDefineSession(sessionUUID)
	if session == nil {
		_ = e.EditResponse(dc.Message{Content: "This session has expired. Please run `/custom-boost-order craft` again.", Ephemeral: true})
		return
	}

	name := sanitizeCustomOrderName(e.TextValue("publish-order-name"))
	if name == "" {
		name = SuggestCustomOrderName(session.template.Lines)
		if name == "" {
			name = "Custom Order"
		}
	}
	session.template.Name = name
	PublishGlobalCustomOrder(session.template)

	contract := FindContract(e.ChannelID())
	if contract == nil && session.contractHash != "" {
		contract = FindContractByHash(session.contractHash)
	}
	msg := BuildDefineCustomOrderMessage(contract, session.template, sessionUUID, fmt.Sprintf("🌍 Published **%s** as a Global Boost Order across all servers!", name))
	_ = e.EditResponse(msg)
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
		Nick:         "Eve (20 Crafts, Alt)",
		IsAlt:        true,
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
		Nick:         "Frank (5 Crafts, Alt)",
		IsAlt:        true,
		IHRRate:      7.5e9,
		TokensWanted: 8,
		TECount:      50,
		ArtifactSet: ArtifactSet{
			Artifacts: []ei.Artifact{{Type: "Deflector", Quality: "None"}},
			LayRate:   14.0,
		},
	}
	farmerstate.SetMiscSettingString("alice", "art_count_29_3_3", "1")
	farmerstate.SetMiscSettingString("alice", "crafts_29_3", "12")
	farmerstate.SetMiscSettingString("bob", "art_count_29_3_3", "0")
	farmerstate.SetMiscSettingString("bob", "crafts_29_3", "85")
	farmerstate.SetMiscSettingString("charlie", "art_count_29_3_3", "0")
	farmerstate.SetMiscSettingString("charlie", "crafts_29_3", "45")
	farmerstate.SetMiscSettingString("dana", "art_count_29_3_3", "0")
	farmerstate.SetMiscSettingString("dana", "crafts_29_3", "20")
	farmerstate.SetMiscSettingString("eve", "art_count_29_3_3", "0")
	farmerstate.SetMiscSettingString("eve", "crafts_29_3", "10")
	farmerstate.SetMiscSettingString("frank", "art_count_29_3_3", "0")
	farmerstate.SetMiscSettingString("frank", "crafts_29_3", "2")

	return c
}

// ⚙️ Preview Message Builder

// BuildDefineCustomOrderMessage creates the ⚙️ report message with table image and action buttons.
func BuildDefineCustomOrderMessage(contract *Contract, tmpl CustomBoostOrderTemplate, sessionUUID string, status string, isSaved ...bool) dc.Message {
	evalContract := contract
	isSample := false
	if evalContract == nil || len(evalContract.Boosters) == 0 {
		evalContract = buildBenchmarkSampleContract()
		isSample = true
	}

	saved := false
	if len(isSaved) > 0 {
		saved = isSaved[0]
	} else if session := getDefineSession(sessionUUID); session != nil {
		saved = session.isSaved
	}

	selectDisabled := !saved || isSample || evalContract == nil

	titleName := tmpl.Name
	if titleName == "" {
		titleName = "Custom Order"
	}

	var headerSb strings.Builder
	fmt.Fprintf(&headerSb, "## ⚙️ Custom Boost Order: **%s**\n", titleName)
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
			Label:    "SELECT",
			Style:    dc.ButtonSuccess,
			CustomID: fmt.Sprintf("%s#%s#select", defineOrderHandlerPrefix, sessionUUID),
			Disabled: selectDisabled,
		},
	}

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

// HandleDefineCustomOrderReactions handles MODIFY, SAVE, PUBLISH, SELECT, DELETE button clicks.
func HandleDefineCustomOrderReactions(client dc.Client, e *dc.ComponentEvent) {
	parts := strings.Split(e.CustomID(), "#")
	if len(parts) < 3 {
		_ = e.Update(dc.Message{Content: "Invalid button interaction.", Ephemeral: true})
		return
	}

	sessionUUID := parts[1]
	action := parts[2]

	if action == "dismiss" || action == "exit" || action == "del_dismiss" {
		clearDefineSession(sessionUUID)
		_ = e.Update(dc.Message{
			Components:      e.MessageComponentsWithoutActionRows(),
			ClearComponents: true,
		})
		return
	}

	session := getDefineSession(sessionUUID)
	if session == nil {
		_ = e.Update(dc.Message{Content: "This session has expired. Please run `/custom-boost-order craft` again.", Ephemeral: true})
		return
	}

	switch action {
	case "del_confirm":
		deletedName := session.template.Name
		if deletedName == "" {
			deletedName = "Custom Order"
		}
		if session.template.Name != "" {
			DeleteUserCustomOrder(session.userID, session.template.Name)
			if session.template.IsGlobal {
				DeleteGlobalCustomOrder(session.template.Name)
			}
		}
		clearDefineSession(sessionUUID)

		components := e.MessageComponentsWithoutActionRows()
		components = append(components, dc.TextDisplay{
			Content: fmt.Sprintf("✅ Custom boost order **%s** has been deleted.", deletedName),
		})
		_ = e.Update(dc.Message{
			Components:      components,
			ClearComponents: true,
		})
		return

	case "modify":
		SendDefineCustomOrderModalFromComponent(e, session.template, sessionUUID)

	case "save":
		SendSaveCustomOrderModal(e, session.template, sessionUUID)

	case "publish":
		SendPublishCustomOrderModal(e, session.template, sessionUUID)

	case "select", "apply":
		_ = e.DeferUpdate()
		contract := FindContract(e.ChannelID())
		if contract == nil && session.contractHash != "" {
			contract = FindContractByHash(session.contractHash)
		}
		if contract != nil {
			contract.mutex.Lock()
			contract.CustomOrderLines = append([]string(nil), session.template.Lines...)
			contract.CustomOrderName = session.template.Name
			contract.BoostOrder = ContractOrderCustom
			unselected := append([]string(nil), contract.Order...)
			contract.Order = sortCustomRemaining(contract, unselected, contract.CustomOrderLines, false)
			contract.mutex.Unlock()

			saveData(contract.ContractHash)
			refreshBoostListMessage(client, contract, false)
			msg := BuildDefineCustomOrderMessage(contract, session.template, sessionUUID, fmt.Sprintf("✅ Selected **%s** as the custom boost order for this contract!", session.template.Name), true)
			_ = e.EditResponse(msg)
			return
		}
		_ = e.EditResponse(dc.Message{Content: "Unable to find active contract to select for.", Ephemeral: true})

	case "delete":
		_ = e.DeferUpdate()
		deletedName := session.template.Name
		if deletedName == "" {
			deletedName = "Custom Order"
		}
		if session.template.Name != "" {
			DeleteUserCustomOrder(session.userID, session.template.Name)
			if session.template.IsGlobal {
				DeleteGlobalCustomOrder(session.template.Name)
			}
		}
		session.template.Name = ""
		session.isSaved = false

		contract := FindContract(e.ChannelID())
		if contract == nil && session.contractHash != "" {
			contract = FindContractByHash(session.contractHash)
		}
		msg := BuildDefineCustomOrderMessage(contract, session.template, sessionUUID, fmt.Sprintf("🗑️ Deleted saved custom boost order **%s**.", deletedName), false)
		_ = e.EditResponse(msg)
		return

	default:
		_ = e.Update(dc.Message{Content: "Unknown button action.", Ephemeral: true})
	}
}
