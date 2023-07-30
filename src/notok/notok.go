package notok

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/sashabaranov/go-openai"
)

type WishStruct struct {
	Wishes []string
	Used   []string
}

var (
	wishes *WishStruct
)

func init() {
	var err error
	wishes, err = loadData()
	if err != nil {
		wishes = new(WishStruct)
	}
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

func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func getWish(w []string) (string, []string) {
	index := rand.Intn(len(w))
	str := w[index]
	w = remove(w, index)
	return str, w
}

func wish(mention string) string {
	var str string = ""

	if len(wishes.Wishes) > 0 {
		str, wishes.Wishes = getWish(wishes.Wishes)
	} else {
		var client = openai.NewClient(config.OpenAIKey)

		var tokenPrompt = "A chicken egg farmer needs a an item of currency, called a token," +
			"to grow his farm. Compose a wish  " +
			"which will bring me a token. The wish should be funny and draw " +
			"from current news events. Start the response of each wish with " +
			"\"Farmer wishes \". The word \"token\" must be used in the response. " +
			"Use gender neutral pronouns. Don't number responses."

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

		if err == nil {
			strmap := strings.Split(resp.Choices[0].Message.Content, "\n")
			for _, el := range strmap {
				if el != "" {
					m1 := regexp.MustCompile(`^.*Farmer`)

					wishes.Wishes = append(wishes.Wishes, m1.ReplaceAllString(el, "Farmer"))
				}
			}
			str, wishes.Wishes = getWish(wishes.Wishes)
		} else {
			//fmt.Println(err.Error()) // Log this
			str, wishes.Used = getWish(wishes.Used)
		}
	}

	wishes.Used = append(wishes.Used, str)
	saveData(wishes)
	name := fmt.Sprintf("**%s**", mention)
	str = strings.Replace(str, "Farmer", name, 1)

	return str
}

func saveData(c *WishStruct) {
	b, _ := json.Marshal(c)
	boost.DataStore.Write("Wishes", b)
}

func loadData() (*WishStruct, error) {
	//diskmutex.Lock()
	var c *WishStruct
	b, err := boost.DataStore.Read("Wishes")
	if err != nil {
		return c, err
	}
	json.Unmarshal(b, &c)
	return c, nil
}
