package notok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/generative-ai-go/genai"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/rs/xid"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)

const aiBotString string = "Eggcellent, the AIrtists have started work and will reply shortly."
const aiTextString string = "Eggcellent, the wrAIters have been tasked with a composition for you.."

var defaultWish = "Draw a balloon animal staring into a lightbulb in an unhealthy way."
var lastWish = defaultWish

func init() {

}

// Notok is the main function for the notok command
func Notok(discord *discordgo.Session, message *discordgo.InteractionCreate, cmd int64, text string) error {
	var name = message.Member.Nick
	if name == "" {
		name = message.Member.User.Username
	}
	var g, err = discord.GuildMember(message.GuildID, message.Member.User.ID)
	if err == nil && g.Nick != "" {
		name = g.Nick
	}

	hidden := true
	wishURL := ""
	wishStr := text
	var aiMsg *discordgo.Message

	// Respond to messages
	var currentStartTime = fmt.Sprintf(" <t:%d:R> ", time.Now().Unix())

	switch cmd {
	case 1:
		aiMsg, err = discord.ChannelMessageSend(message.ChannelID, aiTextString+" "+currentStartTime)
		if err == nil {
			wishStr = wishGemini(name, text)
		}
	case 5: // Open Letter
		aiMsg, err = discord.ChannelMessageSend(message.ChannelID, aiTextString+" "+currentStartTime)
		if err == nil {
			wishStr = letter(name, text)
		}
	case 2:
		aiMsg, err = discord.ChannelMessageSend(message.ChannelID, aiBotString+" "+currentStartTime)
		if err == nil {
			wishStr = letmeout(name, text)
			wishURL = wishImage(wishStr, name, false)
			hidden = false
		}
	case 3:
		str := gonow()
		wishURL = wishImage(str, name, false)
		wishStr = name + " expresses an urgent need to go next up in boost order."
	case 4:
		if len(wishStr) == 0 {
			wishStr = lastWish
		}
		if len(wishStr) < 20 {
			wishStr = defaultWish
		}
		aiMsg, err = discord.ChannelMessageSend(message.ChannelID, aiBotString+" "+currentStartTime)
		if err == nil {
			wishURL = wishImage(wishStr, name, true)
		}
	default:
		return nil
	}

	if aiMsg != nil {
		discord.ChannelMessageDelete(message.ChannelID, aiMsg.ID)
	}

	if err != nil {
		return err
	}

	if wishURL != "" {
		sendImageReply(discord, message.ChannelID, wishURL, wishStr, hidden)
	} else if wishStr != "" {
		discord.ChannelMessageSend(message.ChannelID, wishStr)
		lastWish = wishStr
	} else if wishStr == lastWish {
		lastWish = defaultWish
	}
	return nil
}

// DoGoNow gets the AI to draw a chicken in a hurry
func DoGoNow(s *discordgo.Session, channelID string) {
	var str = gonow()
	s.ChannelTyping(channelID)
	sendImageReply(s, channelID, wishImage(str, "", false), "", false)
}

func sendImageReply(s *discordgo.Session, channelID string, wishURL string, wishStr string, hidden bool) {
	s.ChannelTyping(channelID)
	response, _ := http.Get(wishURL)
	var data discordgo.MessageSend
	if wishStr != lastWish {
		if hidden {
			data.Content = "||" + wishStr + "||"
		} else {
			data.Content = wishStr
		}
	}

	if response != nil && response.StatusCode == 200 {
		var myFile discordgo.File
		myFile.ContentType = "image/png"
		myFile.Name = "ttbb-dalle3.png"
		myFile.Reader = response.Body
		data.Files = append(data.Files, &myFile)
		s.ChannelMessageSendComplex(channelID, &data)
	} else {
		// Error message
		s.ChannelMessageSend(channelID, "Sorry the AIrtists responsed with \""+wishURL+"\"") //"Sorry, the AIrtists are not available at the moment. Some image prompts ")
	}
}

func letter(mention string, text string) string {
	tokenPrompt := "Kevin, the developer of Egg, Inc. has stopped sending widgets to the contract players of his game. Compose a crazy reason requesting that he provide you a widget. The letter should be fairly short and begin with \"Dear Kev,\"."
	tokenPrompt += " " + text
	str, err := getStringFromGoogleGemini(tokenPrompt)
	if err != nil {
		return "Sorry, the AIrtists are not available at the moment."
	}
	m1 := regexp.MustCompile(`\[[A-Za-z\- ]*\]`)
	str = m1.ReplaceAllString(str, "*"+mention+"*")
	str = strings.Replace(str, "widget", "token", -1)

	return str
}

/*
	func getStringFromOpenAI(text string) string {
		var str = ""
		var client = openai.NewClient(config.OpenAIKey)
		var resp, _ = client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: openai.GPT3Dot5Turbo0301,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleUser,
						Content: text,
					},
				},
			},
		)
		str = resp.Choices[0].Message.Content
		return str
	}
*/
func getStringFromGoogleGemini(text string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.GoogleAPIKey))
	if err != nil {
		log.Print(err)
		return "", err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-pro")
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockOnlyHigh,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockOnlyHigh,
		},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
	}
	resp, err := model.GenerateContent(ctx, genai.Text(text))
	if err != nil {
		log.Print(err)
		return "", err
	}

	return printResponse(resp), nil
}

/*
func wish(mention string, text string) string {
	var str = ""
	tokenPrompt := "A contract needs widgets to help with the delivery of eggs. Make a silly wish that would result in a widget being delivered by truck very soon. The response should start with \"I wish\""
	tokenPrompt += " " + text

	str = getStringFromOpenAI(tokenPrompt)
	m1 := regexp.MustCompile(`\[[A-Za-z ]*\]`)
	str = m1.ReplaceAllString(str, "*"+mention+"*")
	str = strings.Replace(str, "widget", "token", -1)
	str = strings.Replace(str, "I wish", mention+" wishes", -1)

	return str
}
*/

func wishGemini(mention string, text string) string {
	tokenPrompt := "A contract needs widgets to help purchase boosts and to share with others to improve speed the delivery of eggs. Make a silly wish that would result in a widget being delivered by truck very soon. The response should be no more than 3 or 4 sentences and start with \"I wish\""
	tokenPrompt += " " + text

	str, err := getStringFromGoogleGemini(tokenPrompt)
	if err != nil {
		return "Sorry, the AIrtists are not available at the moment."
	}
	m1 := regexp.MustCompile(`\[[A-Za-z ]*\]`)
	str = m1.ReplaceAllString(str, "*"+mention+"*")
	str = strings.Replace(str, "widget", "token", -1)
	str = strings.Replace(str, "I wish", mention+" wishes", -1)

	return str
}

func printResponse(resp *genai.GenerateContentResponse) string {
	var str = ""
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
				str += fmt.Sprint(part)
			}
		}
	}
	return str
}

func letmeout(mention string, text string) string {
	var tokenPrompt = //"Using a random city on Earth as the location for this story, don't reuse a previous city choice.  Highlight that city's culture when telling this story about " +
	"A group of chicken egg farmers are locked in their farm " +
		"held hostage by an unknown force. In 250 words or less tell random funny story about this confinement. "
	tokenPrompt += " " + text
	str, err := getStringFromGoogleGemini(tokenPrompt)
	if err != nil {
		return "Sorry, the AIrtists are not available at the moment."
	}

	return str
}

func gonow() string {

	var tokenPrompt = "Compose a scene with a chicken needing blast off quickly and racing towards an outhouse shaped rocket ship " +
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
			Style:          openai.CreateImageStyleNatural,
			User:           user,
		},
	)
	if err != nil {
		var apiError *openai.APIError
		switch {
		case errors.As(err, &apiError):
			return apiError.Message
		default:
			return err.Error()
		}
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
	newfilePrompt := fmt.Sprintf("%s/%s.txt", filepath, id.String())
	os.WriteFile(newfilePrompt, []byte(prompt), 0664)

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
