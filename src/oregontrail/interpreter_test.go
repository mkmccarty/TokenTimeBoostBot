package oregontrail

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
)

func TestParseOregonBas(t *testing.T) {
	data, err := os.ReadFile("../../ttbb-data/oregon.bas")
	if err != nil {
		t.Fatalf("failed to read oregon.bas: %v", err)
	}

	prog, err := ParseProgram(string(data))
	if err != nil {
		t.Fatalf("ParseProgram failed: %v", err)
	}

	if len(prog.Lines) == 0 {
		t.Fatal("expected parsed lines, got 0")
	}

	t.Logf("Parsed %d lines and %d data values successfully", len(prog.Lines), len(prog.DataValues))

	if len(prog.DataValues) != 15 {
		t.Fatalf("expected 15 DATA values, got %d: %v", len(prog.DataValues), prog.DataValues)
	}
}

func TestInterpreterBasicFlow(t *testing.T) {
	data, err := os.ReadFile("../../ttbb-data/oregon.bas")
	if err != nil {
		t.Fatalf("failed to read oregon.bas: %v", err)
	}

	prog, err := ParseProgram(string(data))
	if err != nil {
		t.Fatalf("ParseProgram failed: %v", err)
	}

	env := NewEnv(prog, 12345)
	err = env.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !env.IsWaitingInput() {
		t.Fatal("expected interpreter to be waiting for input (instructions prompt)")
	}

	initialPrompt := env.OutputDrain()
	t.Logf("Initial output:\n%s", initialPrompt)

	if !strings.Contains(initialPrompt, "DO YOU NEED INSTRUCTIONS") {
		t.Fatalf("expected instructions question in output, got: %s", initialPrompt)
	}

	// Answer "NO" to instructions
	err = env.ProvideInput("NO")
	if err != nil {
		t.Fatalf("ProvideInput('NO') failed: %v", err)
	}

	if !env.IsWaitingInput() {
		t.Fatal("expected interpreter to be waiting for input (rifle skill)")
	}

	skillPrompt := env.OutputDrain()
	t.Logf("Skill output:\n%s", skillPrompt)

	if !strings.Contains(skillPrompt, "HOW GOOD A SHOT ARE YOU") {
		t.Fatalf("expected skill prompt, got: %s", skillPrompt)
	}

	// Choose skill 1 (Ace Marksman)
	err = env.ProvideInput("1")
	if err != nil {
		t.Fatalf("ProvideInput('1') failed: %v", err)
	}

	// Oxen prompt
	if !env.IsWaitingInput() {
		t.Fatal("expected interpreter to be waiting for oxen team input")
	}
	oxenPrompt := env.OutputDrain()
	t.Logf("Oxen prompt:\n%s", oxenPrompt)
	if !strings.Contains(oxenPrompt, "OXEN TEAM") {
		t.Fatalf("expected oxen prompt, got: %s", oxenPrompt)
	}

	// Try spending 100 on oxen (too low, should say NOT ENOUGH)
	err = env.ProvideInput("100")
	if err != nil {
		t.Fatalf("ProvideInput('100') failed: %v", err)
	}
	notEnough := env.OutputDrain()
	t.Logf("Too low response:\n%s", notEnough)
	if !strings.Contains(notEnough, "NOT ENOUGH") {
		t.Fatalf("expected NOT ENOUGH, got: %s", notEnough)
	}

	// Provide valid 250 on oxen
	err = env.ProvideInput("250")
	if err != nil {
		t.Fatalf("ProvideInput('250') failed: %v", err)
	}

	// Food prompt
	foodPrompt := env.OutputDrain()
	t.Logf("Food prompt:\n%s", foodPrompt)
	if !strings.Contains(foodPrompt, "FOOD") {
		t.Fatalf("expected food prompt, got: %s", foodPrompt)
	}
	// Spend 200 on food
	_ = env.ProvideInput("200")

	// Ammunition prompt
	ammoPrompt := env.OutputDrain()
	t.Logf("Ammo prompt:\n%s", ammoPrompt)
	// Spend 50 on ammo
	_ = env.ProvideInput("50")

	// Clothing prompt
	clothingPrompt := env.OutputDrain()
	t.Logf("Clothing prompt:\n%s", clothingPrompt)
	// Spend 100 on clothing
	_ = env.ProvideInput("100")

	// Misc supplies prompt
	miscPrompt := env.OutputDrain()
	t.Logf("Misc prompt:\n%s", miscPrompt)
	// Spend 50 on misc
	_ = env.ProvideInput("50")

	// Now purchases should be complete! Total spent: 250 + 200 + 50 + 100 + 50 = 650. Leftover cash: 50.
	turnPrompt := env.OutputDrain()
	t.Logf("First turn output:\n%s", turnPrompt)

	if !strings.Contains(turnPrompt, "AFTER ALL YOUR PURCHASES, YOU NOW HAVE") {
		t.Fatalf("expected purchase summary, got: %s", turnPrompt)
	}
	if !strings.Contains(turnPrompt, "50") {
		t.Fatalf("expected 50 dollars left, got: %s", turnPrompt)
	}
	if !strings.Contains(turnPrompt, "MONDAY MARCH 29 1847") {
		t.Fatalf("expected start date March 29 1847, got: %s", turnPrompt)
	}

	t.Log("Initial turn verified successfully!")
}

func TestInstructions(t *testing.T) {
	data, err := os.ReadFile("../../ttbb-data/oregon.bas")
	if err != nil {
		t.Fatalf("failed to read oregon.bas: %v", err)
	}

	prog, err := ParseProgram(string(data))
	if err != nil {
		t.Fatalf("ParseProgram failed: %v", err)
	}

	env := NewEnv(prog, 12345)
	if err := env.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Answer "YES"
	if err := env.ProvideInput("YES"); err != nil {
		t.Fatalf("ProvideInput('YES') failed: %v", err)
	}

	out := env.OutputDrain()
	if !strings.Contains(out, "THIS PROGRAM SIMULATES A TRIP OVER THE OREGON TRAIL") {
		t.Fatalf("expected instructions text, got: %s", out)
	}
	if !strings.Contains(out, "HOW GOOD A SHOT ARE YOU") {
		t.Fatalf("expected rifle prompt after instructions, got: %s", out)
	}
}

func TestHuntingAndEating(t *testing.T) {
	data, err := os.ReadFile("../../ttbb-data/oregon.bas")
	if err != nil {
		t.Fatalf("failed to read oregon.bas: %v", err)
	}

	prog, err := ParseProgram(string(data))
	if err != nil {
		t.Fatalf("ParseProgram failed: %v", err)
	}

	env := NewEnv(prog, 99999)
	// Mock timer: prompt at t=10.0, response at t=11.2 (1.2 seconds)
	currentTime := 10.0
	env.SetTimerFunc(func() float64 {
		return currentTime
	})

	_ = env.Start()
	_ = env.ProvideInput("NO")
	_ = env.ProvideInput("1")   // Ace Marksman
	_ = env.ProvideInput("250") // Oxen
	_ = env.ProvideInput("200") // Food
	_ = env.ProvideInput("50")  // Ammo
	_ = env.ProvideInput("100") // Clothing
	_ = env.ProvideInput("50")  // Misc

	out := env.OutputDrain()
	t.Logf("Turn 1 options:\n%s", out)

	// In turn 1: "DO YOU WANT TO (1) HUNT, OR (2) CONTINUE"
	// Let's choose 1 (Hunt)
	_ = env.ProvideInput("1")
	huntPrompt := env.OutputDrain()
	t.Logf("Hunt prompt:\n%s", huntPrompt)

	if strings.Contains(huntPrompt, "TYPE ") {
		// Extract word (e.g. BANG, BLAM, POW, WHAM)
		idx := strings.Index(huntPrompt, "TYPE ")
		word := strings.Fields(huntPrompt[idx+5:])[0]
		t.Logf("Need to shoot word: %s", word)

		// Advance mock timer by 1.2 seconds for shooting reaction time
		currentTime += 1.2
		err = env.ProvideInput(word)
		if err != nil {
			t.Fatalf("shoot ProvideInput failed: %v", err)
		}

		shotResult := env.OutputDrain()
		t.Logf("Shot result:\n%s", shotResult)
		if !strings.Contains(shotResult, "DO YOU WANT TO EAT") {
			t.Fatalf("expected eating prompt after shooting, got: %s", shotResult)
		}

		// Choose 3 (Eat well)
		err = env.ProvideInput("3")
		if err != nil {
			t.Fatalf("eat ProvideInput failed: %v", err)
		}

		afterEat := env.OutputDrain()
		t.Logf("After eating turn output:\n%s", afterEat)
	}
}

func TestSessionScrollbackAndInstructions(t *testing.T) {
	sess, err := StartSession("dummy-test-user", "dummy-chan", "")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	// Answer YES to instructions
	err = sess.ProcessInput("YES", 0.0)
	if err != nil {
		t.Fatalf("ProcessInput('YES') failed: %v", err)
	}

	msg := sess.BuildMessage()
	if !msg.ComponentsV1 {
		t.Error("expected ComponentsV1 to be true")
	}
	if len(msg.Embeds) == 0 {
		t.Fatal("expected message to have at least 1 embed")
	}

	desc := msg.Embeds[0].Description
	if !strings.Contains(desc, "THIS PROGRAM SIMULATES A TRIP OVER THE OREGON TRAIL") {
		t.Error("expected embed to contain the start of the instructions")
	}
	if !strings.Contains(desc, "HOW GOOD A SHOT ARE YOU") {
		t.Error("expected embed to contain the rifle prompt following instructions")
	}
	if !strings.Contains(desc, "OXEN - YOU CAN SPEND $200-$300") {
		t.Error("expected embed to contain oxen supplies details")
	}

	t.Logf("Full instruction embed description length: %d characters", len(desc))

	// Verify Guidebook manual
	guideMsg := BuildInstructionsMessage()
	if len(guideMsg.Embeds) == 0 || !strings.Contains(guideMsg.Embeds[0].Description, "OXEN - YOU CAN SPEND $200-$300") {
		t.Error("expected Guidebook manual to contain instructions")
	}
}

func TestBreakOffOldText(t *testing.T) {
	fake := &dctest.FakeClient{}
	sess, err := StartSession("dummy-breakoff-user", "dummy-chan", "")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	// Message 1 exists
	originalMsgID := "msg-12345"
	sess.MessageID = originalMsgID

	// Answer YES to instructions
	_ = sess.ProcessInput("YES", 0.0)

	// Simulate button click on Message 1 with updateFn
	var updatedOldMsg *dc.Message
	updateFn := func(m dc.Message) error {
		updatedOldMsg = &m
		return nil
	}

	sess.UpdateMessageAfterInput(fake, updateFn, originalMsgID)

	// 1. Verify old message was updated with instructions and NO buttons
	if updatedOldMsg == nil {
		t.Fatal("expected old message to be updated via updateFn")
	}
	if len(updatedOldMsg.Components) != 0 {
		t.Errorf("expected old message to have all buttons stripped, got %d components", len(updatedOldMsg.Components))
	}
	if !updatedOldMsg.ClearComponents {
		t.Error("expected ClearComponents to be true on old message to instruct Discord to remove buttons")
	}
	if len(updatedOldMsg.Embeds) == 0 || !strings.Contains(updatedOldMsg.Embeds[0].Description, "THIS PROGRAM SIMULATES A TRIP OVER THE OREGON TRAIL") {
		t.Error("expected old message to contain instructions")
	}

	// 2. Verify NEW message was sent to channel with active prompt and buttons
	if len(fake.SentMessages) != 1 {
		t.Fatalf("expected 1 new active message sent to channel, got %d", len(fake.SentMessages))
	}
	newMsg := fake.SentMessages[0]
	if len(newMsg.Embeds) == 0 || !strings.Contains(newMsg.Embeds[0].Description, "HOW GOOD A SHOT ARE YOU") {
		t.Errorf("expected new message to contain rifle prompt, got: %+v", newMsg.Embeds)
	}
	if len(newMsg.Components) == 0 {
		t.Error("expected new message to have active interactive buttons")
	}

	// 3. Verify sess.MessageID now tracks the NEW message ID (not the original old message ID)
	if sess.MessageID == originalMsgID {
		t.Errorf("sess.MessageID should no longer be old message ID %s", originalMsgID)
	}
	if sess.MessageID == "" {
		t.Error("sess.MessageID should track the newly created message ID")
	}

	// 4. Scrollback should now only contain the rifle question
	if strings.Contains(sess.Scrollback, "THIS PROGRAM SIMULATES A TRIP OVER THE OREGON TRAIL") {
		t.Error("expected session scrollback to no longer contain the instructions")
	}
	if !strings.Contains(sess.Scrollback, "HOW GOOD A SHOT ARE YOU") {
		t.Error("expected session scrollback to retain the current rifle prompt")
	}

	t.Logf("Break off verified: Old msg stripped of buttons, new msg (%s) has active prompt and buttons", sess.MessageID)
}

func TestNumberButtonsForOptions(t *testing.T) {
	sess, err := StartSession("dummy-buttons-user", "dummy-chan", "")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	// Skip instructions
	_ = sess.ProcessInput("NO", 0.0)

	// Waiting for rifle skill (D9)
	comps := sess.BuildMessage().Components
	foundAce := false
	for _, row := range comps {
		if ar, ok := row.(dc.ActionRow); ok {
			for _, c := range ar.Components {
				if b, ok := c.(dc.Button); ok && strings.Contains(b.Label, "Ace") {
					foundAce = true
				}
			}
		}
	}
	if !foundAce {
		t.Error("expected Ace button for rifle skill")
	}

	// Choose 1: Ace
	_ = sess.ProcessInput("1", 0.0)

	// Waiting for Oxen team (A)
	comps = sess.BuildMessage().Components
	found250 := false
	for _, row := range comps {
		if ar, ok := row.(dc.ActionRow); ok {
			for _, c := range ar.Components {
				if b, ok := c.(dc.Button); ok && strings.Contains(b.Label, "250") {
					found250 = true
				}
			}
		}
	}
	if !found250 {
		t.Error("expected $250 button for oxen purchase")
	}

	// Choose $250
	_ = sess.ProcessInput("250", 0.0)

	// Waiting for Food (F)
	comps = sess.BuildMessage().Components
	foundFood200 := false
	for _, row := range comps {
		if ar, ok := row.(dc.ActionRow); ok {
			for _, c := range ar.Components {
				if b, ok := c.(dc.Button); ok && strings.Contains(b.Label, "200") {
					foundFood200 = true
				}
			}
		}
	}
	if !foundFood200 {
		t.Error("expected $200 button for food purchase")
	}

	// Choose $200
	_ = sess.ProcessInput("200", 0.0)

	// Waiting for Ammo (B)
	comps = sess.BuildMessage().Components
	foundAmmo50 := false
	for _, row := range comps {
		if ar, ok := row.(dc.ActionRow); ok {
			for _, c := range ar.Components {
				if b, ok := c.(dc.Button); ok && strings.Contains(b.Label, "50") {
					foundAmmo50 = true
				}
			}
		}
	}
	if !foundAmmo50 {
		t.Error("expected $50 button for ammo purchase")
	}

	t.Log("Number buttons for all initial purchases verified successfully!")
}

func TestStatusLinePurchasesAndUnusedCash(t *testing.T) {
	sess, err := StartSession("test-user-status", "chan-1", "")
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	// Instructions
	if err := sess.ProcessInput("NO", 0.0); err != nil {
		t.Fatalf("failed to pass instructions: %v", err)
	}

	// Rifle skill: 1 (Ace Marksman)
	if err := sess.ProcessInput("1", 0.0); err != nil {
		t.Fatalf("failed to set rifle skill: %v", err)
	}

	// Now at Oxen prompt (waitingVar: A)
	if sess.Env.WaitingVar() != "A" {
		t.Fatalf("expected waitingVar A, got %s", sess.Env.WaitingVar())
	}
	expectedA := "Mileage: 0 mi | Food: $0 | Bullets: 0 | Clothes: $0 | Misc: $0 | Cash: $700"
	if sl := sess.StatusLine(); sl != expectedA {
		t.Errorf("Oxen prompt status line mismatch:\ngot:  %s\nwant: %s", sl, expectedA)
	}

	// Answer Oxen: $200
	if err := sess.ProcessInput("200", 0.0); err != nil {
		t.Fatalf("failed to enter oxen: %v", err)
	}

	// Now at Food prompt (waitingVar: F)
	if sess.Env.WaitingVar() != "F" {
		t.Fatalf("expected waitingVar F, got %s", sess.Env.WaitingVar())
	}
	expectedF := "Mileage: 0 mi | Food: $0 | Bullets: 0 | Clothes: $0 | Misc: $0 | Cash: $500"
	if sl := sess.StatusLine(); sl != expectedF {
		t.Errorf("Food prompt status line mismatch:\ngot:  %s\nwant: %s", sl, expectedF)
	}

	// Answer Food: $100
	if err := sess.ProcessInput("100", 0.0); err != nil {
		t.Fatalf("failed to enter food: %v", err)
	}

	// Now at Ammunition prompt (waitingVar: B)
	if sess.Env.WaitingVar() != "B" {
		t.Fatalf("expected waitingVar B, got %s", sess.Env.WaitingVar())
	}
	expectedB := "Mileage: 0 mi | Food: $100 | Bullets: 0 | Clothes: $0 | Misc: $0 | Cash: $400"
	if sl := sess.StatusLine(); sl != expectedB {
		t.Errorf("Ammo prompt status line mismatch:\ngot:  %s\nwant: %s", sl, expectedB)
	}

	// Answer Ammunition: $25 (giving 1250 bullets)
	if err := sess.ProcessInput("25", 0.0); err != nil {
		t.Fatalf("failed to enter ammo: %v", err)
	}

	// Now at Clothing prompt (waitingVar: C)
	if sess.Env.WaitingVar() != "C" {
		t.Fatalf("expected waitingVar C, got %s", sess.Env.WaitingVar())
	}
	expectedC := "Mileage: 0 mi | Food: $100 | Bullets: 1250 | Clothes: $0 | Misc: $0 | Cash: $375"
	if sl := sess.StatusLine(); sl != expectedC {
		t.Errorf("Clothes prompt status line mismatch:\ngot:  %s\nwant: %s", sl, expectedC)
	}

	// Answer Clothing: $75
	if err := sess.ProcessInput("75", 0.0); err != nil {
		t.Fatalf("failed to enter clothes: %v", err)
	}

	// Now at Miscellaneous prompt (waitingVar: M1)
	if sess.Env.WaitingVar() != "M1" {
		t.Fatalf("expected waitingVar M1, got %s", sess.Env.WaitingVar())
	}
	expectedM1 := "Mileage: 0 mi | Food: $100 | Bullets: 1250 | Clothes: $75 | Misc: $0 | Cash: $300"
	if sl := sess.StatusLine(); sl != expectedM1 {
		t.Errorf("Misc prompt status line mismatch:\ngot:  %s\nwant: %s", sl, expectedM1)
	}

	// Answer Misc: $75
	if err := sess.ProcessInput("75", 0.0); err != nil {
		t.Fatalf("failed to enter misc: %v", err)
	}

	// Turn 1 reached (waitingVar: X)
	expectedTurn1 := "Mileage: 0 mi | Food: $100 | Bullets: 1250 | Clothes: $75 | Misc: $75 | Cash: $225"
	if sl := sess.StatusLine(); sl != expectedTurn1 {
		t.Errorf("Turn 1 status line mismatch:\ngot:  %s\nwant: %s", sl, expectedTurn1)
	}

	// Verify BuildMessage reflects this in content and footer
	msg := sess.BuildMessage()
	expectedContent := fmt.Sprintf("📊 `%s`", expectedTurn1)
	if msg.Content != expectedContent {
		t.Errorf("BuildMessage content mismatch:\ngot:  %s\nwant: %s", msg.Content, expectedContent)
	}
	if len(msg.Embeds) == 0 || msg.Embeds[0].Footer == nil || msg.Embeds[0].Footer.Text != expectedTurn1 {
		t.Errorf("BuildMessage footer mismatch:\ngot:  %+v\nwant: %s", msg.Embeds[0].Footer, expectedTurn1)
	}

	t.Logf("Status line successfully verified through all purchase questions: %s", expectedTurn1)
}

func TestHandleOregonTrailCommand_AddsUserToThread(t *testing.T) {
	fake := dctest.New()
	guildID := "9001"
	channelID := "8001"
	userID := "123456789012345678"

	fake.WithChannel(channelID, guildID, "general")
	fake.WithPermissions(config.DiscordAppID, channelID, dc.Permissions(1<<35))

	payload := fmt.Sprintf(`{
		"id": "123",
		"token": "tok",
		"guild_id": %q,
		"channel": {"id": %q, "type": 0},
		"member": {"user": {"id": %q, "username": "PioneerTest", "discriminator": "0"}},
		"user": {"id": %q, "username": "PioneerTest", "discriminator": "0"},
		"data": {"id": "1", "name": "oregon-trail", "type": 1, "options": []}
	}`, guildID, channelID, userID, userID)

	cmdEvent, err := dc.NewCommandEventFromPayload([]byte(payload))
	if err != nil {
		t.Fatalf("failed to create command event: %v", err)
	}

	HandleOregonTrailCommand(fake, cmdEvent)

	// Verify StartThread was called and AddThreadMember was called for the user
	calls := fake.AllCalls()
	foundStartThread := false
	foundAddThreadMember := false
	for _, call := range calls {
		if call.Method == "StartThread" {
			foundStartThread = true
		}
		if call.Method == "AddThreadMember" {
			if len(call.Args) >= 2 && call.Args[1] == userID {
				foundAddThreadMember = true
			}
		}
	}

	if !foundStartThread {
		t.Error("expected StartThread to be called")
	}
	if !foundAddThreadMember {
		t.Errorf("expected AddThreadMember to be called for user %s, calls: %+v", userID, calls)
	}
}
