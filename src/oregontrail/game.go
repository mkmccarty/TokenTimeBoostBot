package oregontrail

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// SessionTimeout is how long an idle game session is kept before expiry.
const SessionTimeout = 24 * time.Hour

// OregonTrailInstructions is the full original manual from the 1975 BASIC source.
const OregonTrailInstructions = `THIS PROGRAM SIMULATES A TRIP OVER THE OREGON TRAIL FROM
INDEPENDENCE, MISSOURI TO OREGON CITY, OREGON IN 1847.
YOUR FAMILY OF FIVE WILL COVER THE 2040 MILE OREGON TRAIL
IN 5-6 MONTHS --- IF YOU MAKE IT ALIVE.

YOU HAD SAVED $900 TO SPEND FOR THE TRIP, AND YOU'VE JUST
   PAID $200 FOR A WAGON.
YOU WILL NEED TO SPEND THE REST OF YOUR MONEY ON THE
   FOLLOWING ITEMS:

     OXEN - YOU CAN SPEND $200-$300 ON YOUR TEAM
            THE MORE YOU SPEND, THE FASTER YOU'LL GO
               BECAUSE YOU'LL HAVE BETTER ANIMALS

     FOOD - THE MORE YOU HAVE, THE LESS CHANCE THERE
               IS OF GETTING SICK

     AMMUNITION - $1 BUYS A BELT OF 50 BULLETS
            YOU WILL NEED BULLETS FOR ATTACKS BY ANIMALS
               AND BANDITS, AND FOR HUNTING FOOD

     CLOTHING - THIS IS ESPECIALLY IMPORTANT FOR THE COLD
               WEATHER YOU WILL ENCOUNTER WHEN CROSSING
               THE MOUNTAINS

     MISCELLANEOUS SUPPLIES - THIS INCLUDES MEDICINE AND
               OTHER THINGS YOU WILL NEED FOR SICKNESS
               AND EMERGENCY REPAIRS

YOU CAN SPEND ALL YOUR MONEY BEFORE YOU START YOUR TRIP -
OR YOU CAN SAVE SOME OF YOUR CASH TO SPEND AT FORTS ALONG
THE WAY WHEN YOU RUN LOW. HOWEVER, ITEMS COST MORE AT
THE FORTS. YOU CAN ALSO GO HUNTING ALONG THE WAY TO GET
MORE FOOD.

WHENEVER YOU HAVE TO USE YOUR TRUSTY RIFLE ALONG THE WAY,
YOU WILL BE TOLD TO TYPE IN A WORD (ONE THAT SOUNDS LIKE A
GUN SHOT). THE FASTER YOU TYPE IN THAT WORD AND HIT THE
RETURN KEY, THE BETTER LUCK YOU'LL HAVE WITH YOUR GUN.

AT EACH TURN, ALL ITEMS ARE SHOWN IN DOLLAR AMOUNTS
EXCEPT BULLETS.
WHEN ASKED TO ENTER MONEY AMOUNTS, DON'T USE A "$".

GOOD LUCK!!!`

// GameSession tracks the active state of an Oregon Trail game for one user.
type GameSession struct {
	ID          string
	UserID      string
	ChannelID   string
	ThreadID    string
	MessageID   string
	Env         *Env
	LastPrompt  string
	Scrollback  string
	PromptTime  time.Time
	LastActive  time.Time
	ShootWord   string
	BaseTimer   float64
	SimDuration float64
}

var (
	sessionMu sync.Mutex
	sessions  = make(map[string]*GameSession) // Key: UserID
)

func randomSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetSession returns the active session for a user, or nil if expired/missing.
func GetSession(userID string) *GameSession {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	s, ok := sessions[userID]
	if !ok {
		return nil
	}
	if time.Since(s.LastActive) > SessionTimeout {
		delete(sessions, userID)
		return nil
	}
	return s
}

// StartSession creates and initializes a new game session.
func StartSession(userID, channelID, threadID string) (*GameSession, error) {
	prog, err := GetProgram()
	if err != nil {
		return nil, err
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()

	env := NewEnv(prog, time.Now().UnixNano())
	sess := &GameSession{
		ID:         randomSessionID(),
		UserID:     userID,
		ChannelID:  channelID,
		ThreadID:   threadID,
		Env:        env,
		LastActive: time.Now(),
		BaseTimer:  1000.0,
	}

	// Wire TimerFunc to reflect real reaction time for shooting
	env.SetTimerFunc(func() float64 {
		return sess.BaseTimer + sess.SimDuration
	})

	if err := env.Start(); err != nil {
		return nil, fmt.Errorf("failed to start game: %w", err)
	}

	sess.LastPrompt = env.OutputDrain()
	sess.Scrollback = sess.LastPrompt
	sess.PromptTime = time.Now()
	sess.updateShootWord()

	sessions[userID] = sess
	return sess, nil
}

// EndSession removes the user's active session.
func EndSession(userID string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(sessions, userID)
}

// ProcessInput advances the game with user input string.
func (s *GameSession) ProcessInput(input string, reactionSeconds float64) error {
	s.LastActive = time.Now()
	s.SimDuration = reactionSeconds

	err := s.Env.ProvideInput(input)
	if err != nil {
		return err
	}

	newOutput := s.Env.OutputDrain()
	s.LastPrompt = newOutput

	if strings.TrimSpace(input) != "" {
		s.Scrollback += fmt.Sprintf("\n> %s\n", input)
	}
	s.Scrollback += newOutput
	s.PromptTime = time.Now()
	s.updateShootWord()

	if s.Env.IsTerminated() {
		EndSession(s.UserID)
	}
	return nil
}

var shootWordRegex = regexp.MustCompile(`\bTYPE\s+([A-Z]+)\b`)

func (s *GameSession) updateShootWord() {
	s.ShootWord = ""
	upper := strings.ToUpper(s.LastPrompt)
	m := shootWordRegex.FindStringSubmatch(upper)
	if len(m) > 1 {
		s.ShootWord = m[1]
	}
}

// CheckBreakOff checks if the accumulated output has become too long or if instructions finished.
// If so, it splits the text, sets s.Scrollback to the remaining active prompt,
// and returns hasBreak=true with oldMsg containing the older text and NO components.
func (s *GameSession) CheckBreakOff() (bool, dc.Message) {
	clean := cleanOutputText(s.Scrollback, 10000)

	// Case 1: If instructions were printed, break off the instructions block into its own message
	if strings.Contains(clean, "THIS PROGRAM SIMULATES A TRIP OVER THE OREGON TRAIL") {
		rifleIdx := strings.Index(clean, "HOW GOOD A SHOT ARE YOU")
		if rifleIdx != -1 {
			oldText := strings.TrimSpace(clean[:rifleIdx])
			currentText := strings.TrimSpace(clean[rifleIdx:])

			s.Scrollback = currentText
			return true, dc.Message{
				ComponentsV1:    true,
				ClearComponents: true,
				Embeds: []dc.Embed{
					{
						Title:       "📜 Oregon Trail Instructions (1847)",
						Description: "```\n" + oldText + "\n```",
						Color:       0x2E8B57, // Trail Green
						Footer: &dc.EmbedFooter{
							Text: "Original 1975 CDC Cyber 70 BASIC • Bill Heinemann, Don Rawitsch, Paul Dillenberger",
						},
					},
				},
				Components: nil,
			}
		}
	}

	// Case 2: Output scrollback has grown too long (> 1200 characters)
	if len(clean) > 1200 {
		targetKeep := 500
		if len(clean) > targetKeep {
			splitIdx := len(clean) - targetKeep
			if relIdx := strings.Index(clean[splitIdx:], "\n\n"); relIdx != -1 {
				splitIdx += relIdx + 2
			} else if relIdx := strings.Index(clean[splitIdx:], "\n"); relIdx != -1 {
				splitIdx += relIdx + 1
			}

			if splitIdx > 0 && splitIdx < len(clean) {
				oldText := strings.TrimSpace(clean[:splitIdx])
				currentText := strings.TrimSpace(clean[splitIdx:])

				if oldText != "" && currentText != "" {
					s.Scrollback = currentText
					return true, dc.Message{
						ComponentsV1:    true,
						ClearComponents: true,
						Embeds: []dc.Embed{
							{
								Title:       "🛞 Oregon Trail - Journey Log",
								Description: "```\n" + oldText + "\n```",
								Color:       0x8B5A2B,
							},
						},
						Components: nil,
					}
				}
			}
		}
	}

	return false, dc.Message{}
}

// UpdateMessageAfterInput updates the Discord interface after processing an input step.
// If older text needs to be broken off:
// 1. The original/old message is updated to contain only the archived text and components are removed.
// 2. A new message is created at the bottom of the channel with the most recent text and buttons.
// 3. The session's MessageID is updated to this new message ID so subsequent button interactions attach to it.
func (s *GameSession) UpdateMessageAfterInput(client dc.Client, updateFn func(dc.Message) error, currentMsgID string) {
	prevMsgID := s.MessageID
	if currentMsgID != "" {
		prevMsgID = currentMsgID
		s.MessageID = currentMsgID
	}

	hasBreak, oldMsg := s.CheckBreakOff()
	if hasBreak {
		oldMsg.ComponentsV1 = true
		oldMsg.ClearComponents = true
		oldMsg.Components = nil

		// 1. Update the original old message so it only shows old historical text, with buttons stripped.
		if updateFn != nil {
			_ = updateFn(oldMsg)
		} else if client != nil && s.ChannelID != "" && prevMsgID != "" {
			_, _ = client.EditMessage(s.ChannelID, prevMsgID, oldMsg)
		} else if client != nil && s.ChannelID != "" {
			_, _ = client.SendMessage(s.ChannelID, oldMsg)
		}

		// 2. Send the new message with the most recent text and interactive buttons.
		if client != nil && s.ChannelID != "" {
			newMsg, err := client.SendMessage(s.ChannelID, s.BuildMessage())
			if err == nil && newMsg != nil {
				s.MessageID = newMsg.ID
			}
		}

		// 3. Clear buttons from previous message if it exists and was not already updated
		if prevMsgID != "" && prevMsgID != s.MessageID && updateFn == nil && client != nil && s.ChannelID != "" {
			_, _ = client.EditMessage(s.ChannelID, prevMsgID, dc.Message{
				ComponentsV1:    true,
				ClearComponents: true,
			})
		}
	} else {
		// No break-off: update the current message in place with new text and buttons.
		if updateFn != nil {
			_ = updateFn(s.BuildMessage())
		} else if client != nil && s.ChannelID != "" && s.MessageID != "" {
			_, _ = client.EditMessage(s.ChannelID, s.MessageID, s.BuildMessage())
		}
	}
}

// BreakOffOldText performs break-off of older text if needed.
func (s *GameSession) BreakOffOldText(client dc.Client) {
	s.UpdateMessageAfterInput(client, nil, s.MessageID)
}

// BuildMessageWithoutComponents returns the message view with all interactive buttons removed.
func (s *GameSession) BuildMessageWithoutComponents() dc.Message {
	msg := s.BuildMessage()
	msg.Components = nil
	msg.ClearComponents = true
	return msg
}

// StatusLine formats the current funds and supplies status line, calculating unused cash
// during initial purchases and tracking real balances throughout the game.
func (s *GameSession) StatusLine() string {
	m := s.Env.NumVar("M")
	f := s.Env.NumVar("F")
	b := s.Env.NumVar("B")
	c := s.Env.NumVar("C")
	m1 := s.Env.NumVar("M1")
	t := s.Env.NumVar("T")
	a := s.Env.NumVar("A")

	waitingVar := s.Env.WaitingVar()

	var bullets float64
	var cash float64
	food := f
	clothes := c
	misc := m1

	switch waitingVar {
	case "A":
		// Asking for oxen: nothing spent yet from $700 budget
		cash = 700
		food = 0
		bullets = 0
		clothes = 0
		misc = 0
	case "F":
		// Oxen purchased: subtract A
		cash = 700 - a
		food = 0
		bullets = 0
		clothes = 0
		misc = 0
	case "B":
		// Oxen and food purchased: subtract A + F
		cash = 700 - a - f
		bullets = 0
		clothes = 0
		misc = 0
	case "C":
		// Oxen, food, ammo purchased: B is ammo dollars spent ($1 = 50 bullets)
		cash = 700 - a - f - b
		bullets = b * 50
		clothes = 0
		misc = 0
	case "M1":
		// Oxen, food, ammo, clothes purchased
		cash = 700 - a - f - b - c
		bullets = b * 50
		misc = 0
	default:
		// Active game turns or after purchases
		bullets = b
		cash = t
	}

	if cash < 0 {
		cash = 0
	}

	return fmt.Sprintf("Mileage: %.0f mi | Food: $%.0f | Bullets: %.0f | Clothes: $%.0f | Misc: $%.0f | Cash: $%.0f",
		m, food, bullets, clothes, misc, cash)
}

// BuildMessage formats the current game screen into rich Discord embeds and interactive buttons.
func (s *GameSession) BuildMessage() dc.Message {
	cleanText := cleanOutputText(s.Scrollback, 3800)
	if cleanText == "" {
		if s.Env.IsTerminated() {
			cleanText = "The journey has ended."
		} else {
			cleanText = "Starting the trail..."
		}
	}

	embed := dc.Embed{
		Title:       "🛞 Oregon Trail (1975 BASIC)",
		Description: "```\n" + cleanText + "\n```",
		Color:       0x8B5A2B, // Pioneer leather brown
	}

	statusLine := s.StatusLine()
	waitingVar := s.Env.WaitingVar()
	isInitialPurchase := waitingVar == "A" || waitingVar == "F" || waitingVar == "B" || waitingVar == "C" || waitingVar == "M1"
	hasStartedTrail := s.Env.NumVar("M") > 0 || s.Env.NumVar("T") > 0 || s.Env.NumVar("A") > 0

	content := ""
	if s.Env.IsTerminated() {
		embed.Footer = &dc.EmbedFooter{
			Text: "Game Over • Use /oregon-trail to start a new adventure",
		}
	} else if isInitialPurchase || hasStartedTrail {
		embed.Footer = &dc.EmbedFooter{
			Text: statusLine,
		}
		content = fmt.Sprintf("📊 `%s`", statusLine)
	} else {
		embed.Footer = &dc.EmbedFooter{
			Text: "Original 1975 CDC Cyber 70 BASIC • Bill Heinemann, Don Rawitsch, Paul Dillenberger",
		}
	}

	components := s.buildComponents()

	return dc.Message{
		Content:      content,
		ComponentsV1: true,
		Embeds:       []dc.Embed{embed},
		Components:   components,
	}
}

func cleanOutputText(s string, maxChars int) string {
	lines := strings.Split(s, "\n")
	var result []string
	emptyCount := 0
	for _, l := range lines {
		trimmed := strings.TrimRight(l, " \t\r")
		if trimmed == "" {
			emptyCount++
			if emptyCount <= 1 {
				result = append(result, "")
			}
		} else {
			emptyCount = 0
			result = append(result, trimmed)
		}
	}
	out := strings.Join(result, "\n")
	if len(out) > maxChars {
		out = out[len(out)-maxChars:]
		if idx := strings.Index(out, "\n"); idx != -1 {
			out = out[idx+1:]
		}
	}
	return out
}

func (s *GameSession) buildComponents() []dc.LayoutComponent {
	if s.Env.IsTerminated() {
		return []dc.LayoutComponent{
			dc.ActionRow{
				Components: []dc.InteractiveComponent{
					dc.Button{
						Label:    "Play Again",
						CustomID: fmt.Sprintf("ot#new#%s", s.ID),
						Style:    dc.ButtonSuccess,
					},
				},
			},
		}
	}

	var rows []dc.LayoutComponent
	upper := strings.ToUpper(s.LastPrompt)
	waitingVar := s.Env.WaitingVar()

	// 1. Shooting prompt: reaction button
	if s.ShootWord != "" {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    fmt.Sprintf("💥 %s!", s.ShootWord),
					CustomID: fmt.Sprintf("ot#shoot#%s#%s", s.ShootWord, s.ID),
					Style:    dc.ButtonDanger,
				},
				dc.Button{
					Label:    "Type Word",
					CustomID: fmt.Sprintf("ot#modal#%s", s.ID),
					Style:    dc.ButtonSecondary,
				},
			},
		})
		return rows
	}

	// 2. Yes/No prompts
	if waitingVar == "C$" && (strings.Contains(upper, "YES/NO") || strings.Contains(upper, "INFORM YOUR NEXT OF KIN")) {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Yes",
					CustomID: fmt.Sprintf("ot#choice#YES#%s", s.ID),
					Style:    dc.ButtonPrimary,
				},
				dc.Button{
					Label:    "No",
					CustomID: fmt.Sprintf("ot#choice#NO#%s", s.ID),
					Style:    dc.ButtonSecondary,
				},
				dc.Button{
					Label:    "📜 Manual",
					CustomID: fmt.Sprintf("ot#guide#%s", s.ID),
					Style:    dc.ButtonSecondary,
				},
				dc.Button{
					Label:    "Quit",
					CustomID: fmt.Sprintf("ot#quit#%s", s.ID),
					Style:    dc.ButtonDanger,
				},
			},
		})
		return rows
	}

	// 3. Rifle expertise level (1-5)
	if waitingVar == "D9" && strings.Contains(upper, "HOW GOOD A SHOT") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "1: Ace", CustomID: fmt.Sprintf("ot#choice#1#%s", s.ID), Style: dc.ButtonSuccess},
				dc.Button{Label: "2: Good", CustomID: fmt.Sprintf("ot#choice#2#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "3: Fair", CustomID: fmt.Sprintf("ot#choice#3#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "4: Practice", CustomID: fmt.Sprintf("ot#choice#4#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "5: Shaky", CustomID: fmt.Sprintf("ot#choice#5#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 4. Oxen purchase options ($200 - $300)
	if waitingVar == "A" && strings.Contains(upper, "OXEN") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "$200", CustomID: fmt.Sprintf("ot#choice#200#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$220", CustomID: fmt.Sprintf("ot#choice#220#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$250 (Best)", CustomID: fmt.Sprintf("ot#choice#250#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "$280", CustomID: fmt.Sprintf("ot#choice#280#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$300", CustomID: fmt.Sprintf("ot#choice#300#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "✍️ Other Amount", CustomID: fmt.Sprintf("ot#modal#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 5. Food purchase options
	if waitingVar == "F" && strings.Contains(upper, "FOOD") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "$100", CustomID: fmt.Sprintf("ot#choice#100#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$150", CustomID: fmt.Sprintf("ot#choice#150#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$200", CustomID: fmt.Sprintf("ot#choice#200#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "$250", CustomID: fmt.Sprintf("ot#choice#250#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$300", CustomID: fmt.Sprintf("ot#choice#300#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "✍️ Other Amount", CustomID: fmt.Sprintf("ot#modal#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 6. Ammunition purchase options ($1 = 50 bullets)
	if waitingVar == "B" && strings.Contains(upper, "AMMUNITION") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "$10 (500)", CustomID: fmt.Sprintf("ot#choice#10#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$25 (1250)", CustomID: fmt.Sprintf("ot#choice#25#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$50 (2500)", CustomID: fmt.Sprintf("ot#choice#50#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "$75 (3750)", CustomID: fmt.Sprintf("ot#choice#75#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$100 (5000)", CustomID: fmt.Sprintf("ot#choice#100#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "✍️ Other Amount", CustomID: fmt.Sprintf("ot#modal#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 7. Clothing purchase options
	if waitingVar == "C" && strings.Contains(upper, "CLOTHING") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "$25", CustomID: fmt.Sprintf("ot#choice#25#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$50", CustomID: fmt.Sprintf("ot#choice#50#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$75", CustomID: fmt.Sprintf("ot#choice#75#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "$100", CustomID: fmt.Sprintf("ot#choice#100#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$125", CustomID: fmt.Sprintf("ot#choice#125#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "✍️ Other Amount", CustomID: fmt.Sprintf("ot#modal#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 8. Miscellaneous supplies purchase options
	if waitingVar == "M1" && strings.Contains(upper, "MISCELLANEOUS") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "$20", CustomID: fmt.Sprintf("ot#choice#20#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$35", CustomID: fmt.Sprintf("ot#choice#35#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$50", CustomID: fmt.Sprintf("ot#choice#50#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "$75", CustomID: fmt.Sprintf("ot#choice#75#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$100", CustomID: fmt.Sprintf("ot#choice#100#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "✍️ Other Amount", CustomID: fmt.Sprintf("ot#modal#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 9. Fort purchases (P)
	if waitingVar == "P" {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "$0 (Skip)", CustomID: fmt.Sprintf("ot#choice#0#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$10", CustomID: fmt.Sprintf("ot#choice#10#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "$20", CustomID: fmt.Sprintf("ot#choice#20#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$30", CustomID: fmt.Sprintf("ot#choice#30#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "$50", CustomID: fmt.Sprintf("ot#choice#50#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "✍️ Other Amount", CustomID: fmt.Sprintf("ot#modal#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 10. Turn options: (1) Stop at Fort, (2) Hunt, or (3) Continue
	if waitingVar == "X" && strings.Contains(upper, "STOP AT THE NEXT FORT") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "1: Fort", CustomID: fmt.Sprintf("ot#choice#1#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "2: Hunt", CustomID: fmt.Sprintf("ot#choice#2#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "3: Continue", CustomID: fmt.Sprintf("ot#choice#3#%s", s.ID), Style: dc.ButtonSuccess},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 11. Turn options: (1) Hunt, or (2) Continue
	if waitingVar == "X" && strings.Contains(upper, "(1) HUNT, OR (2) CONTINUE") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "1: Hunt", CustomID: fmt.Sprintf("ot#choice#1#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "2: Continue", CustomID: fmt.Sprintf("ot#choice#2#%s", s.ID), Style: dc.ButtonSuccess},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 12. Eating choices: (1) Poorly, (2) Moderately, (3) Well
	if waitingVar == "E" && strings.Contains(upper, "POORLY") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "1: Poorly", CustomID: fmt.Sprintf("ot#choice#1#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "2: Moderately", CustomID: fmt.Sprintf("ot#choice#2#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "3: Well", CustomID: fmt.Sprintf("ot#choice#3#%s", s.ID), Style: dc.ButtonSuccess},
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 13. Attack tactics: (1) Run (2) Attack (3) Continue/Ignore (4) Circle Wagons
	if waitingVar == "T1" && strings.Contains(upper, "TACTICS") {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "1: Run", CustomID: fmt.Sprintf("ot#choice#1#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "2: Attack", CustomID: fmt.Sprintf("ot#choice#2#%s", s.ID), Style: dc.ButtonDanger},
				dc.Button{Label: "3: Ignore", CustomID: fmt.Sprintf("ot#choice#3#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "4: Circle Wagons", CustomID: fmt.Sprintf("ot#choice#4#%s", s.ID), Style: dc.ButtonPrimary},
			},
		})
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "📜 Manual", CustomID: fmt.Sprintf("ot#guide#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "Quit", CustomID: fmt.Sprintf("ot#quit#%s", s.ID), Style: dc.ButtonDanger},
			},
		})
		return rows
	}

	// 14. Fallback buttons
	if isNumericVar(waitingVar) {
		rows = append(rows, dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{Label: "0", CustomID: fmt.Sprintf("ot#choice#0#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "1", CustomID: fmt.Sprintf("ot#choice#1#%s", s.ID), Style: dc.ButtonPrimary},
				dc.Button{Label: "2", CustomID: fmt.Sprintf("ot#choice#2#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "3", CustomID: fmt.Sprintf("ot#choice#3#%s", s.ID), Style: dc.ButtonSecondary},
				dc.Button{Label: "4", CustomID: fmt.Sprintf("ot#choice#4#%s", s.ID), Style: dc.ButtonSecondary},
			},
		})
	}

	rows = append(rows, dc.ActionRow{
		Components: []dc.InteractiveComponent{
			dc.Button{
				Label:    "✍️ Enter Custom",
				CustomID: fmt.Sprintf("ot#modal#%s", s.ID),
				Style:    dc.ButtonSecondary,
			},
			dc.Button{
				Label:    "📜 Manual",
				CustomID: fmt.Sprintf("ot#guide#%s", s.ID),
				Style:    dc.ButtonSecondary,
			},
			dc.Button{
				Label:    "Quit",
				CustomID: fmt.Sprintf("ot#quit#%s", s.ID),
				Style:    dc.ButtonDanger,
			},
		},
	})

	return rows
}

func isNumericVar(v string) bool {
	return !strings.HasSuffix(v, "$")
}

// BuildInputModal creates a popup modal for text or numeric entry.
func (s *GameSession) BuildInputModal() dc.Modal {
	label := "Enter your choice or amount:"
	placeholder := "e.g. 200"

	waitingVar := s.Env.WaitingVar()
	switch waitingVar {
	case "A":
		label = "Oxen team expense ($200-$300):"
		placeholder = "250"
	case "F":
		label = "Food expense ($):"
		placeholder = "200"
	case "B":
		label = "Ammunition expense ($):"
		placeholder = "50"
	case "C":
		label = "Clothing expense ($):"
		placeholder = "100"
	case "M1":
		label = "Misc supplies expense ($):"
		placeholder = "50"
	case "P":
		label = "Fort purchase amount ($):"
		placeholder = "20"
	default:
		if strings.HasSuffix(waitingVar, "$") {
			label = "Enter text:"
			placeholder = "YES or NO"
		}
	}

	return dc.Modal{
		CustomID: fmt.Sprintf("ot#submit#%s", s.ID),
		Title:    "Oregon Trail Input",
		Inputs: []dc.TextInput{
			{
				CustomID:    "value",
				Label:       label,
				Style:       dc.TextInputStyleShort,
				Placeholder: placeholder,
				Required:    true,
				MinLength:   1,
				MaxLength:   20,
			},
		},
	}
}

// BuildInstructionsMessage returns an embed containing the full original instructions manual.
func BuildInstructionsMessage() dc.Message {
	return dc.Message{
		ComponentsV1: true,
		Ephemeral:    true,
		Embeds: []dc.Embed{
			{
				Title:       "📜 Oregon Trail (1847) — Guidebook",
				Description: "```\n" + OregonTrailInstructions + "\n```",
				Color:       0x2E8B57, // Trail Forest Green
				Footer: &dc.EmbedFooter{
					Text: "Original 1975 BASIC Game Instructions • Minnesota Educational Computing Consortium",
				},
			},
		},
	}
}
