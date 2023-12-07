package notok

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/rs/xid"
	"github.com/sashabaranov/go-openai"
)

const AIBOT_STRING string = "Eggcellent, the AIrtists have started work and will reply shortly."
const AIBOTTXT_STRING string = "Eggcellent, the wrAIters have been tasked with a composition for you.."

var lastWish = "Draw a balloon animal staring into a lightbulb in an unhealthy way."

func init() {

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

	wishUrl := ""
	wishStr := ""
	var aiMsg *discordgo.Message
	// Respond to messages
	var currentStartTime = fmt.Sprintf(" <t:%d:R> ", time.Now().Unix())

	switch {
	//case strings.HasPrefix(message.Content, "!notoki"):
	//	discord.ChannelMessageDelete(message.ChannelID, message.ID)
	//	discord.ChannelTyping(message.ChannelID)
	//	wishStr = wish(name)
	//	wishUrl = wishImage(wishStr+" Represent this using creepy cryptid chickens in the style of a 5 year olds crayon drawing.", name, false)
	case strings.HasPrefix(message.Content, "!notok"):
		discord.ChannelMessageDelete(message.ChannelID, message.ID)
		aiMsg, _ = discord.ChannelMessageSend(message.ChannelID, AIBOTTXT_STRING+" "+currentStartTime)
		wishStr = wish(name)
	case strings.HasPrefix(message.Content, "!letmeout"):
		discord.ChannelMessageDelete(message.ChannelID, message.ID)
		aiMsg, _ = discord.ChannelMessageSend(message.ChannelID, AIBOT_STRING+" "+currentStartTime)
		wishStr = letmeout(name)
		wishUrl = wishImage(wishStr, name, false)
	case strings.HasPrefix(message.Content, "!gonow"):
		discord.ChannelMessageDelete(message.ChannelID, message.ID)
		str := gonow()
		wishUrl = wishImage(str, name, false)
	case strings.HasPrefix(message.Content, "!draw"):
		discord.ChannelMessageDelete(message.ChannelID, message.ID)
		wishStr = strings.TrimPrefix(message.Content, "!draw ")
		if wishStr == "!draw" {
			wishStr = lastWish
		}
		aiMsg, _ = discord.ChannelMessageSend(message.ChannelID, AIBOT_STRING+" "+currentStartTime)
		wishUrl = wishImage(wishStr, name, true)
	}

	if aiMsg != nil {
		discord.ChannelMessageDelete(message.ChannelID, aiMsg.ID)
	}

	if wishUrl != "" {
		discord.ChannelTyping(message.ChannelID)
		response, _ := http.Get(wishUrl)
		//discord.ChannelFileSend(message.ChannelID, "BB-img.png", response.Body)
		var data discordgo.MessageSend
		data.Content = wishStr
		var myFile discordgo.File
		myFile.ContentType = "image/png"
		myFile.Name = "ttbb-dalle3.png"
		myFile.Reader = response.Body
		data.Files = append(data.Files, &myFile)

		discord.ChannelMessageSendComplex(message.ChannelID, &data)
	} else if wishStr != "" {
		discord.ChannelMessageSend(message.ChannelID, wishStr)
	}
	if wishStr != "" {
		lastWish = wishStr
	}

}

func DoGoNow(discord *discordgo.Session, channelID string) {
	var str = gonow()
	discord.ChannelTyping(channelID)
	discord.ChannelMessageSend(channelID, wishImage(str, "", false))
}

func wish(mention string) string {
	var str string = ""

	var client = openai.NewClient(config.OpenAIKey)

	tokenPrompt = "Kevin, the developer of Egg, Inc. has stopped sending widgets to the contract players of his game. Compose a crazy reason requesting that he provide you a widget. The leter should begin with \"Dear Kev,\"."

	var resp, _ = client.CreateChatCompletion(
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
	m1 := regexp.MustCompile(`\[[A-Za-z ]*\]`)
	str = m1.ReplaceAllString(resp.Choices[0].Message.Content, "*"+mention+"*")
	str = strings.Replace(str, "widget", "token", -1)

	return str
}

func letmeout(mention string) string {
	var str string = ""

	var client = openai.NewClient(config.OpenAIKey)

	var tokenPrompt = //"Using a random city on Earth as the location for this story, don't reuse a previous city choice.  Highlight that city's culture when telling this story about " +
	"a group of chicken egg farmers are locked in their farm " +
		"held hostage by an unknown force. In 100 words tell random funny story about this confinement. " +
		"Use gender neutral pronouns."

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

	if err != nil {
		str = ""
	} else {
		str = resp.Choices[0].Message.Content
	}

	return str
}

func gonow() string {

	var tokenPrompt = "Compose a scene with a chicken farmer needing blast off into space and racing off towards an outhouse shaped rocket ship " +
		"in a comical cartoonish environment exaggerating the farmer's urgency."

	tokenPrompt = "Compose a scene with a chicken needing blast off quickly and racing towards an outhouse shaped rocket ship " +
		"in a comical cartoonish environment exaggerating the urgency."

	return tokenPrompt
}

func wishImage(prompt string, user string, retry bool) string {
	var client = openai.NewClient(config.OpenAIKey)

	respURL, err := client.CreateImage(
		context.Background(),
		openai.ImageRequest{
			Prompt:         fmt.Sprintf("%s %s", prompt, ""),
			Model:          openai.CreateImageModelDallE3,
			N:              1,
			Size:           openai.CreateImageSize1024x1024,
			ResponseFormat: openai.CreateImageResponseFormatURL,
			Quality:        openai.CreateImageQualityStandard,
			Style:          openai.CreateImageStyleVivid,
			User:           user,
		},
	)
	if err != nil {
		if !retry {
			return "No image returned. Many times this is an OpenAI content filter that blocks the image. Try again."
		}
		fmt.Printf("Image creation error: %v\n", err)
		return wishImage(prompt, user, false)
	}

	fmt.Println(prompt)
	fmt.Println(respURL.Data[0].URL)

	go downloadFile("./ttbb-data/images", respURL.Data[0].URL, prompt)

	return respURL.Data[0].URL
}

func downloadFile(filepath string, url string, prompt string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = os.MkdirAll(filepath, os.ModePerm)
	if err != nil {
		return err
	}

	id := xid.New()
	newfile := fmt.Sprintf("%s/%s.png", filepath, id.String())
	newfile_prompt := fmt.Sprintf("%s/%s.txt", filepath, id.String())
	os.WriteFile(newfile_prompt, []byte(prompt), 0664)

	// Create the file
	out, err := os.Create(newfile)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
