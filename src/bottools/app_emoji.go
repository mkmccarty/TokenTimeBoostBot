package bottools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"golang.org/x/image/draw"
)

// ExploreEmoji will list all the emojis in the app
func ExploreEmoji(s *discordgo.Session) {

	appEmoji, err := s.ApplicationEmojis(config.DiscordAppID)
	if err != nil {
		log.Print(err)
		return
	}
	for _, e := range appEmoji {
		emoji, err := s.ApplicationEmoji(config.DiscordAppID, e.ID)
		if err != nil {
			log.Print(err)
			return
		}
		log.Print(emoji.Name, emoji.ID, emoji.Animated)
	}
	// eparam.Name = "test"
	/*
		s.ApplicationEmojiCreate(config.DiscordAppID, "test", "https://cdn.discordapp.com/emojis/1234567890.png", nil)
		s.ApplicationEmojiDelete(config.DiscordAppID, "test2")

	*/

	/*
			var builder strings.Builder
			builder.WriteString("URL for the following is all /")

			for _, e := range appEmoji {
				builder.WriteString(fmt.Sprintf("%s,http://cdn.discordapp.com/emojis/%s.png\n", e.Name, e.ID))
			}
			log.Print(builder.String())
		*
			/*
				u, _ := s.UserChannelCreate(config.AdminUserID)
				var data discordgo.MessageSend
				data.Content = builder.String()
				data.Embed = &discordgo.MessageEmbed{
					Title:       "emoji",
					Description: "Images",
				}

				_, err = s.ChannelMessageSendComplex(u.ID, &data)
				if err != nil {
					log.Print(err)
				}
	*/
}

// ImportEggImage will import an egg image into the discord app
func ImportEggImage(s *discordgo.Session, eggID, IconURL string) (string, error) {

	// Read the icon URL into memory
	resp, err := http.Get(IconURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	iconData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var src image.Image
	src, _, err = image.Decode(strings.NewReader(string(iconData)))
	if err != nil {
		return "", err
	}

	// Create a new variable for egg.ID, but remove spaces and hyphens from its name and make it lowercase
	cleanEggID := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(eggID), " ", ""), "-", "")
	// Do something with iconData if needed
	destinationImage := image.NewRGBA(image.Rect(0, 0, src.Bounds().Max.X/4, src.Bounds().Max.Y/4))
	draw.NearestNeighbor.Scale(destinationImage, destinationImage.Rect, src, src.Bounds(), draw.Over, nil)
	//output, _ := os.Create(fmt.Sprintf("egg_%s.png", cleanEggID))
	//defer output.Close()
	//png.Encode(output, destinationImage)

	var buf bytes.Buffer
	err = png.Encode(&buf, destinationImage)
	if err != nil {
		return "", err
	}
	base64Image := base64.StdEncoding.EncodeToString(buf.Bytes())

	//
	const DiscordURIPng = "data:image/png;base64,"
	//const DiscordURIJpg = "data:image/jpeg;base64,"
	//const DiscordURIGif = "data:image/gif;base64,"

	data := discordgo.EmojiParams{
		Name:  fmt.Sprintf("mkm_%s", cleanEggID),
		Image: DiscordURIPng + base64Image,
	}

	newID, err := s.ApplicationEmojiCreate(config.DiscordAppID, &data)
	if err != nil {
		log.Print(err)
		return "", err
	}
	emojiData := ei.EggEmojiData{
		Name:  newID.Name,
		ID:    newID.ID,
		DevID: newID.ID,
	}
	ei.EggEmojiMap[strings.ToUpper(eggID)] = emojiData
	return fmt.Sprintf(":%s:%s:", newID.Name, newID.ID), nil
}
