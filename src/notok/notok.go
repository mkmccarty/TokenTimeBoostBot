package notok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"

	"github.com/bwmarrin/discordgo"
	"github.com/google/generative-ai-go/genai"
	"github.com/rs/xid"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)

const googleModel = "gemini-2.0-flash-lite"

var defaultWish = "Show a potato staring into a lightbulb in an unhealthy way."

var lastWish = defaultWish

func init() {

}

// Notok is the main function for the notok command
func Notok(s *discordgo.Session, i *discordgo.InteractionCreate, cmd int64, text string) error {
	var name = i.Member.Nick
	if name == "" {
		name = i.Member.User.GlobalName
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
	contractDesc := boost.GetContractDescription(i.ChannelID)

	switch cmd {
	case 1:

		wishStr, err = wishGemini(name, text, contractDesc)
	case 5: // Open Letter
		wishStr, err = letter(name, text, contractDesc)
	case 2:
		wishStr, err = getLetMeOutString(text, contractDesc)
		if err == nil {
			wishURL, err = wishImage(wishStr, name)
		}
		hidden = false
	case 3:
		str := gonow(contractDesc)
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
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: fmt.Sprintf("%s\nResponse time: %s", err.Error(), time.Since(currentStartTime).Round(time.Second).String()),
			},
		)
		return err
	}

	if wishURL != "" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true,
			&discordgo.WebhookParams{
				Content: fmt.Sprintf("Success\nResponse time: %s", time.Since(currentStartTime).Round(time.Second).String()),
			},
		)
		sendImageReply(s, i.ChannelID, wishURL, wishStr, hidden)
	} else if wishStr != "" {
		if strings.HasPrefix(text, "!!") {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: wishStr},
			)
		} else {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: fmt.Sprintf("Success\nResponse time: %s", time.Since(currentStartTime).Round(time.Second).String()),
				},
			)
			_, _ = s.ChannelMessageSend(i.ChannelID, wishStr)
			lastWish = wishStr
		}
	} else if wishStr == lastWish {
		lastWish = defaultWish
	}
	return nil
}

// DoGoNow gets the AI to draw a chicken in a hurry
func DoGoNow(s *discordgo.Session, channelID string) {
	var str = gonow(boost.GetContractDescription(channelID))
	_ = s.ChannelTyping(channelID)
	wishURL, _ := wishImage(str, "")
	sendImageReply(s, channelID, wishURL, "", false)
}

func sendImageReply(s *discordgo.Session, channelID string, wishURL string, wishStr string, hidden bool) {
	_ = s.ChannelTyping(channelID)
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
		_, _ = s.ChannelMessageSendComplex(channelID, &data)
	} else {
		// Error message
		_, _ = s.ChannelMessageSend(channelID, "Sorry the AIrtists responsed with \""+wishURL+"\"") //"Sorry, the AIrtists are not available at the moment. Some image prompts ")
	}
}

func letter(mention string, text string, desc string) (string, error) {
	var builder strings.Builder

	builder.WriteString("Your role is a farmer of chickens who needs the delivery of widgets to help with the delivery of their chicken eggs for a contract.")
	if desc != "" {
		builder.WriteString(desc + ".")
	}
	builder.WriteString("Kevin, the developer of Egg, Inc. has stopped sending widgets to the contract players of his game. ")
	builder.WriteString("Compose a crazy reason requesting that he provide you a widget. The letter should be fairly short and begin with \"Dear Kev,\".")
	builder.WriteString(mention + " would like suggest this additional consideration when composing the letter \"" + text + "\".")
	builder.WriteString("The letter should be signed as if sent by " + mention + ".")
	builder.WriteString("The response should replace any form of use of \"widget\" with the appropriate \"token\".")

	str, err := getStringFromGoogleGemini(builder.String())
	if err != nil {
		return "", err
	}

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
	defer func() {
		if err := client.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
	model := client.GenerativeModel(googleModel)
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

	respStr := printResponse(resp, true)
	if strings.HasPrefix(respStr, "I'm sorry, but") {
		return "", errors.New(respStr)
	}

	return printResponse(resp, false), nil
}

// GetContractTeamNames returns a list of team names for a given contract prompt.
func GetContractTeamNames(prompt string, quantity int) []string {
	if config.GoogleAPIKey == "" {
		return nil
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "My Egg Inc contract today wants \"%s\". Return a list of %d team names in a comma separated list with no other context.", prompt, quantity)

	str, err := getStringFromGoogleGemini(builder.String())
	if err != nil {
		return nil
	}

	// Want to split the string result, trim whitespace for each split string, and remove any empty strings
	strArray := strings.Split(str, ",")
	for i, s := range strArray {
		strArray[i] = strings.TrimSpace(s)
	}
	return strArray

}

func wishGemini(mention string, text string, desc string) (string, error) {
	var builder strings.Builder
	if !strings.HasPrefix(text, "!!") {
		builder.WriteString("Your role is a farmer of chickens who needs the delivery of widgets to help with the delivery of their chicken eggs for a contract.")
		if desc != "" {
			builder.WriteString(desc + ".")
		}
		builder.WriteString("Compose a randomly comical or desparate wish that could result in a widget being delivered by truck very soon.")
		builder.WriteString("The response should should be no more than 5 sentences. ")
		builder.WriteString("The wish should start with \"" + mention + " wishes\".")
		builder.WriteString("In the response replace any form of use of the word \"widget\" with an appropriate form of \"token\".")
		builder.WriteString(mention + " would like suggest this additional consideration for adjusting the response \"" + text + "\".")
		//builder.WriteString("The response should should be no more than 5 sentences and start with \"I wish\"")
	} else {
		builder.WriteString(text[2:])
	}

	str, err := getStringFromGoogleGemini(builder.String())
	if err != nil {
		return "", err
	}

	return str, err
}

func printResponse(resp *genai.GenerateContentResponse, logit bool) string {
	var str = ""
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				if logit {
					log.Println(part)
				}
				str += fmt.Sprint(part)
			}
		}
	}

	str = strings.ReplaceAll(str, "widget", "token")

	return str
}

func getLetMeOutString(text string, desc string) (string, error) {
	var builder strings.Builder
	builder.WriteString("Your role is a farmer of chickens who needs the delivery of widgets to help with the delivery of their chicken eggs for a contract.")
	if desc != "" {
		builder.WriteString(desc + ".")
	}

	builder.WriteString("A group of chickens are locked in their farm held hostage by an unknown force.\n")
	builder.WriteString("In 150 words or less tell a funny story about this confinement.\n")
	builder.WriteString(text)
	str, err := getStringFromGoogleGemini(builder.String())
	if err != nil {
		return "", err
	}

	return str, nil
}

func gonow(desc string) string {
	var builder strings.Builder
	builder.WriteString("Your role is a typical American chicken farmer who is working to deliver their chicken eggs for a contract.")
	if desc != "" {
		builder.WriteString(desc + ".")
	}

	builder.WriteString("Compose a scene where one of those chickens needs to quickly find an outhouse shaped like a rocket ship " +
		"in a comical cartoonish environment exaggerating the urgency.")

	return builder.String()
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

	go downloadFileNoErr("./ttbb-data/images", respURL.Data[0].URL, prompt)

	return respURL.Data[0].URL, nil
}

func downloadFileNoErr(filepath string, url string, prompt string) {
	_ = downloadFile(filepath, url, prompt)
}

func downloadFile(filepath string, url string, prompt string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	err = os.MkdirAll(filepath, os.ModePerm)
	if err != nil {
		return err
	}

	id := xid.New()
	newfile := fmt.Sprintf("%s/%s.png", filepath, id.String())
	newfilePrompt := fmt.Sprintf("%s/%s.txt", filepath, id.String())
	err = os.WriteFile(newfilePrompt, []byte(prompt), 0664)
	if err != nil {
		log.Print(err)
	}

	// Create the file
	out, err := os.Create(newfile)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()
	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
