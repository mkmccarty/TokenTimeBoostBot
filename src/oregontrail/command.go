package oregontrail

import (
	"fmt"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashOregonTrailCommand returns the slash command definition for /oregon-trail.
func GetSlashOregonTrailCommand(cmd string) *dc.Command {
	return &dc.Command{
		Name:        cmd,
		Description: "Play the original 1975 BASIC version of Oregon Trail.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.StringOption{
				Name:        "action",
				Description: "Start a new game, resume existing, or quit",
				Required:    false,
				Choices: []dc.Choice[string]{
					{Name: "Start New Game", Value: "start"},
					{Name: "Resume Game", Value: "resume"},
					{Name: "Read Guide / Instructions", Value: "guide"},
					{Name: "Quit Current Game", Value: "quit"},
				},
			},
		},
	}
}

// HandleOregonTrailCommand processes the /oregon-trail slash command.
func HandleOregonTrailCommand(client dc.Client, e *dc.CommandEvent) {
	userID := e.UserID()
	channelID := e.ChannelID()
	action := ""
	if v, ok := e.OptString("action"); ok {
		action = v
	}

	if action == "guide" {
		_ = e.Respond(BuildInstructionsMessage())
		return
	}

	existing := GetSession(userID)

	if action == "quit" {
		if existing != nil {
			EndSession(userID)
			_ = e.Respond(dc.Message{
				Content:   "You abandoned your wagon on the trail. Game ended.",
				Ephemeral: true,
			})
		} else {
			_ = e.Respond(dc.Message{
				Content:   "You don't have an active Oregon Trail game.",
				Ephemeral: true,
			})
		}
		return
	}

	if existing != nil && action != "start" {
		// Resume existing game
		_ = e.Respond(existing.BuildMessage())
		return
	}

	// Determine if we should start in a thread for clean guild channel layout
	targetChannelID := channelID
	threadID := ""
	if e.GuildID() != "" {
		// In a guild text channel, check if we can create a thread
		ch, err := client.Channel(channelID)
		if err == nil && !ch.IsThread {
			perms, permErr := client.UserChannelPermissions(config.DiscordAppID, channelID)
			if permErr == nil && perms.CreatePublicThreads() {
				userName := e.User().Username
				if userName == "" {
					userName = "Pioneer"
				}
				threadName := fmt.Sprintf("🛞 Oregon Trail - %s", userName)
				if len(threadName) > 100 {
					threadName = threadName[:100]
				}
				th, startErr := client.StartThread(channelID, threadName, 1440)
				if startErr == nil {
					targetChannelID = th.ID
					threadID = th.ID
					_ = client.JoinThread(th.ID)
					_ = client.AddThreadMember(th.ID, userID)
				}
			}
		}
	}

	sess, err := StartSession(userID, targetChannelID, threadID)
	if err != nil {
		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("Failed to load Oregon Trail: %v", err),
			Ephemeral: true,
		})
		return
	}

	msg := sess.BuildMessage()
	if threadID != "" && threadID != channelID {
		// Started inside a thread: announce in channel and send start message in thread
		_ = e.Respond(dc.Message{
			Content: fmt.Sprintf("🛞 Your Oregon Trail expedition has begun in <#%s>!", threadID),
		})
		_ = client.AddThreadMember(threadID, userID)
		sentMsg, err := client.SendMessage(threadID, msg)
		if err == nil && sentMsg != nil {
			sess.MessageID = sentMsg.ID
		}
	} else {
		_ = e.Respond(msg)
	}
}

// HandleComponent processes button clicks originating from Oregon Trail messages.
func HandleComponent(client dc.Client, e *dc.ComponentEvent) {
	customID := e.CustomID()
	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		return
	}

	action := parts[1]
	userID := e.UserID()
	sess := GetSession(userID)

	if action == "new" {
		sess, err := StartSession(userID, e.ChannelID(), "")
		if err != nil {
			_ = e.Respond(dc.Message{
				Content:   fmt.Sprintf("Failed to restart game: %v", err),
				Ephemeral: true,
			})
			return
		}
		sess.MessageID = e.MessageID()
		_ = e.Update(sess.BuildMessage())
		return
	}

	if sess == nil {
		_ = e.Respond(dc.Message{
			Content:   "Your game session has expired or does not exist. Use `/oregon-trail` to start a new adventure!",
			Ephemeral: true,
		})
		return
	}

	switch action {
	case "choice":
		// Format: ot#choice#<val>#<sessID>
		if len(parts) < 3 {
			return
		}
		choiceVal := parts[2]
		if err := sess.ProcessInput(choiceVal, 0.0); err != nil {
			_ = e.Respond(dc.Message{Content: fmt.Sprintf("Input error: %v", err), Ephemeral: true})
			return
		}
		sess.UpdateMessageAfterInput(client, e.Update, e.MessageID())

	case "shoot":
		// Format: ot#shoot#<word>#<sessID>
		if len(parts) < 3 {
			return
		}
		word := parts[2]
		reaction := time.Since(sess.PromptTime).Seconds()
		if err := sess.ProcessInput(word, reaction); err != nil {
			_ = e.Respond(dc.Message{Content: fmt.Sprintf("Shoot error: %v", err), Ephemeral: true})
			return
		}
		sess.UpdateMessageAfterInput(client, e.Update, e.MessageID())

	case "modal":
		_ = e.ShowModal(sess.BuildInputModal())

	case "guide":
		_ = e.Respond(BuildInstructionsMessage())

	case "quit":
		EndSession(userID)
		_ = e.Update(dc.Message{
			Content:    "**🛞 OREGON TRAIL**\nYou retired to the nearest settlement and abandoned the trail. Better luck next time!",
			Components: nil,
		})
	}
}

// HandleModal processes submitted text input modals.
func HandleModal(client dc.Client, e *dc.ModalEvent) {
	customID := e.CustomID()
	parts := strings.Split(customID, "#")
	if len(parts) < 2 {
		return
	}

	userID := e.UserID()
	sess := GetSession(userID)
	if sess == nil {
		_ = e.Respond(dc.Message{
			Content:   "Your game session has expired. Start a new game with `/oregon-trail`.",
			Ephemeral: true,
		})
		return
	}

	val := e.TextValue("value")
	reaction := time.Since(sess.PromptTime).Seconds()

	if err := sess.ProcessInput(val, reaction); err != nil {
		_ = e.Respond(dc.Message{Content: fmt.Sprintf("Error processing input: %v", err), Ephemeral: true})
		return
	}

	sess.UpdateMessageAfterInput(client, e.Update, e.MessageID())
}

// HandleMessage allows users to respond to prompts by sending text messages in active threads/DMs.
func HandleMessage(client dc.Client, e *dc.MessageEvent) {
	if e.AuthorIsBot() {
		return
	}
	content := strings.TrimSpace(e.Content())
	if content == "" {
		return
	}

	sess := GetSession(e.AuthorID())
	if sess == nil {
		return
	}

	// Verify channel matches
	if sess.ThreadID != "" && e.ChannelID() != sess.ThreadID {
		return
	} else if sess.ThreadID == "" && e.ChannelID() != sess.ChannelID {
		return
	}

	reaction := time.Since(sess.PromptTime).Seconds()
	if err := sess.ProcessInput(content, reaction); err != nil {
		return
	}

	hasBreak, oldMsg := sess.CheckBreakOff()
	if hasBreak {
		if sess.MessageID != "" {
			_, _ = client.EditMessage(sess.ChannelID, sess.MessageID, oldMsg)
		}
	} else {
		if sess.MessageID != "" {
			_, _ = client.EditMessage(sess.ChannelID, sess.MessageID, sess.BuildMessageWithoutComponents())
		}
	}

	newMsg, err := client.SendMessage(e.ChannelID(), sess.BuildMessage())
	if err == nil && newMsg != nil {
		sess.MessageID = newMsg.ID
	}
}
