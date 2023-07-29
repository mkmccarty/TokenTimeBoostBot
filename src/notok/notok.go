package notok

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/sashabaranov/go-openai"
)

var (
	wishes []string
)

func init() {
	wishes, _ = loadData()
}

func Notok(discord *discordgo.Session, message *discordgo.MessageCreate) {

	// Ignore bot messaage
	if message.Author.ID == discord.State.User.ID {
		return
	}
	var name = message.Author.Username
	var g, err = discord.GuildMember(message.GuildID, message.Author.ID)
	if err == nil && g.Nick != "" {
		name = g.Nick
	}

	// Respond to messages
	switch {
	case strings.HasPrefix(message.Content, "!notok"):
		discord.ChannelMessageSend(message.ChannelID, wish(name))
	}
}

func wish(mention string) string {
	var client = openai.NewClient(config.OpenAIKey)

	var tokenPrompt = "A chicken egg farmer needs a an item of currency, called a token," +
		"to grow his farm. Compose a wish  " +
		"which will bring me a token.  The wish should be funny and draw " +
		"from current news events. Start the response of each wish with " +
		"\"Farmer wishes \".  The word \"token\" must be used in the response. " +
		"Use gender neural pronouns. Don't number responses."

	//var tokenPrompt = "A chicken farmer needs tokens to be successful on his farm. He finds a bottle with a genie who will grant him wishes. Tell me 3 wishes to ask for. Start the response of each wish with \"Farmer wishes \". Respond in a JSON format."
	var resp, err = client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo0301,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: tokenPrompt,
				},
			},
		},
	)

	var str string = ""
	if err != nil {
		// Probably problem with permissions
		str = wishes[rand.Intn(len(wishes))]
	} else {
		strmap := strings.Split(resp.Choices[0].Message.Content, "\n")
		for _, el := range strmap {
			if el != "" {

				wishes = append(wishes, el)
			}
		}
		saveData(wishes)
		str = strings.Replace(wishes[len(wishes)-1], "Farmer", mention, 1)
	}

	return str
}

func saveData(c []string) {
	b, _ := json.Marshal(c)
	boost.DataStore.Write("Wishes", b)
}

func loadData() ([]string, error) {
	//diskmutex.Lock()
	var c []string
	b, err := boost.DataStore.Read("Wishes")
	if err != nil {
		return c, err
	}
	json.Unmarshal(b, &c)
	return c, nil
}
