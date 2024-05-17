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
const aiTextString string = "Eggcellent, the wrAIters have been tasked with a composition for you."

// var defaultWish = "Draw a balloon animal staring into a lightbulb in an unhealthy way."
var defaultWish = "Show a potato staring into a lightbulb in an unhealthy way."

var lastWish = defaultWish

func init() {

}

// Notok is the main function for the notok command
func Notok(s *discordgo.Session, i *discordgo.InteractionCreate, cmd int64, text string) error {
	var name = i.Member.Nick
	if name == "" {
		name = i.Member.User.Username
	}
	var g, err = s.GuildMember(i.GuildID, i.Member.User.ID)
	if err == nil && g.Nick != "" {
		name = g.Nick
	}

	hidden := true
	wishURL := ""
	wishStr := text

	// Respond to messages
	var currentStartTime = time.Now()

	switch cmd {
	case 1:
		wishStr, err = wishGemini(name, text)
	case 5: // Open Letter
		wishStr, err = letter(name, text)
	case 2:
		wishStr, err = getLetMeOutString(text)
		if err == nil {
			wishURL, err = wishImage(wishStr, name)
		}
		hidden = false
	case 3:
		str := gonow()
		wishURL, err = wishImage(str, name)
		wishStr = name + " expresses an urgent need to go next up in boost order."
	case 4:
		if len(wishStr) == 0 {
			wishStr = lastWish
		}
		if len(wishStr) < 20 {
			wishStr = defaultWish
		}
		wishURL, err = wishImage(wishStr, name)
	default:
		return nil
	}

	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: fmt.Sprintf("%s\nResponse time: %s", err.Error(), time.Since(currentStartTime).Round(time.Second).String()),
			},
		)
		return err
	}

	if wishURL != "" {
		s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: fmt.Sprintf("Success\nResponse time: %s", time.Since(currentStartTime).Round(time.Second).String()),
			},
		)
		sendImageReply(s, i.ChannelID, wishURL, wishStr, hidden)
	} else if wishStr != "" {
		if strings.HasPrefix(text, "!!") {
			s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: wishStr},
			)
		} else {
			s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: fmt.Sprintf("Success\nResponse time: %s", time.Since(currentStartTime).Round(time.Second).String()),
				},
			)
			s.ChannelMessageSend(i.ChannelID, wishStr)
			lastWish = wishStr
		}
	} else if wishStr == lastWish {
		lastWish = defaultWish
	}
	return nil
}

// DoGoNow gets the AI to draw a chicken in a hurry
func DoGoNow(s *discordgo.Session, channelID string) {
	var str = gonow()
	s.ChannelTyping(channelID)
	wishURL, _ := wishImage(str, "")
	sendImageReply(s, channelID, wishURL, "", false)
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

func letter(mention string, text string) (string, error) {
	tokenPrompt := "Kevin, the developer of Egg, Inc. has stopped sending widgets to the contract players of his game. Compose a crazy reason requesting that he provide you a widget. The letter should be fairly short and begin with \"Dear Kev,\"."
	tokenPrompt += " " + text
	str, err := getStringFromGoogleGemini(tokenPrompt)
	if err != nil {
		return "", err
	}

	m1 := regexp.MustCompile(`\[[A-Za-z\- ]*\]`)
	str = m1.ReplaceAllString(str, "*"+mention+"*")
	str = strings.Replace(str, "widget", "token", -1)

	return str, nil
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
			Threshold: genai.HarmBlockOnlyHigh},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockOnlyHigh},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockOnlyHigh},
	}
	resp, err := model.GenerateContent(ctx, genai.Text(text))
	if err != nil {
		log.Print(err)
		return "", err
	}

	respStr := printResponse(resp)
	if strings.HasPrefix(respStr, "I'm sorry, but") {
		return "", errors.New(respStr)
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

func wishGemini(mention string, text string) (string, error) {
	var builder strings.Builder
	if !strings.HasPrefix(text, "!!") {
		builder.WriteString("A contract needs widgets to help purchase boosts and to share with others to improve speed the delivery of eggs.")
		builder.WriteString("Make a silly wish that would result in a widget being delivered by truck very soon.")
		builder.WriteString("The response should should be no more than 4 sentences and start with \"I wish\"")
		builder.WriteString(text)
	} else {
		builder.WriteString(text[2:])
	}

	str, err := getStringFromGoogleGemini(builder.String())
	if err != nil {
		return "", err
	}
	m1 := regexp.MustCompile(`\[[A-Za-z ]*\]`)
	str = m1.ReplaceAllString(str, "*"+mention+"*")
	str = strings.Replace(str, "widget", "token", -1)
	str = strings.Replace(str, "I wish", mention+" wishes", -1)

	return str, err
}

func printResponse(resp *genai.GenerateContentResponse) string {
	var str = ""
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				log.Println(part)
				str += fmt.Sprint(part)
			}
		}
	}
	return str
}

func getLetMeOutString(text string) (string, error) {
	var builder strings.Builder
	builder.WriteString("A group of chickens are locked in their farm held hostage by an unknown force.\n")
	builder.WriteString("In 150 words or less tell a funny story about this confinement.\n")
	builder.WriteString(text)
	str, err := getStringFromGoogleGemini(builder.String())
	if err != nil {
		return "", err
	}

	return str, nil
}

func gonow() string {

	var tokenPrompt = "Compose a scene with a chicken needing blast off quickly and racing towards an outhouse shaped rocket ship " +
		"in a comical cartoonish environment exaggerating the urgency."

	return tokenPrompt
}

func wishImage(prompt string, user string) (string, error) {
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
		var apiError *openai.APIError
		switch {
		case errors.As(err, &apiError):
			return "", errors.New(apiError.Message)
		default:
			return "", errors.New("error creating image")
		}
	}

	fmt.Println(prompt)
	fmt.Println(respURL.Data[0].URL)

	go downloadFile("./ttbb-data/images", respURL.Data[0].URL, prompt)

	return respURL.Data[0].URL, nil
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
