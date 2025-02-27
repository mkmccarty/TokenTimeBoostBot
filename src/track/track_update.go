package track

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// HandleTrackerRefresh will call the API and update the start time and duration
func HandleTrackerRefresh(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID string
	if i.GuildID != "" {
		userID = i.Member.User.ID
	} else {
		userID = i.User.ID
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "Checking Coop Status for start time and duration...",
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
	if err != nil {
		log.Println(err)
	}

	name := extractTokenName(i.MessageComponentData().CustomID)
	if name == "" {
		name = extractTokenNameOriginal(i.Message.Components[0])
	}

	// Update the tracker with new values
	t, err := getTrack(userID, name)
	if err == nil {
		startTime, contractDurationSeconds, err := DownloadCoopStatusTracker(t.ContractID, t.CoopID)
		if err != nil {
			errorStr := fmt.Sprintf("Error: %s, check your coop-id", err)
			_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &errorStr,
			})
			return
		}
		t.TimeFromCoopStatus = time.Now()
		embed := TokenTrackingAdjustTime(i.ChannelID, userID, name, 0, 0, 0, 0, startTime, contractDurationSeconds)
		comp := getTokenValComponents(t.Name, t.Linked && !t.LinkedCompleted)
		m := discordgo.NewMessageEdit(t.UserChannelID, t.TokenMessageID)
		m.Components = &comp
		m.SetEmbeds(embed.Embeds)
		m.SetContent("")
		_, _ = s.ChannelMessageEditComplex(m)
	}
	_ = s.InteractionResponseDelete(i.Interaction)

	saveData(Tokens)
}

// DownloadCoopStatusTracker will download the coop status for a given contract and coop ID
func DownloadCoopStatusTracker(contractID string, coopID string) (time.Time, float64, error) {
	nowTime := time.Now()

	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return time.Time{}, 0, fmt.Errorf("Invalid contract ID")
	}

	coopStatus, _, _, err := ei.GetCoopStatus(contractID, coopID)
	if err != nil {
		return time.Time{}, 0, err
	}

	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return time.Time{}, 0, fmt.Errorf("%s", ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())])
	}
	var contractDurationSeconds float64
	var calcSecondsRemaining int64

	grade := int(coopStatus.GetGrade())

	startTime := nowTime
	secondsRemaining := int64(coopStatus.GetSecondsRemaining())
	endTime := nowTime
	if coopStatus.GetSecondsSinceAllGoalsAchieved() > 0 {
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		secondsSinceAllGoals := int64(coopStatus.GetSecondsSinceAllGoalsAchieved())
		endTime = endTime.Add(-time.Duration(secondsSinceAllGoals) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
	} else {
		var totalContributions float64
		var contributionRatePerSecond float64
		// Need to figure out how much longer this contract will run
		for _, c := range coopStatus.GetContributors() {
			totalContributions += c.GetContributionAmount()
			totalContributions += -(c.GetContributionRate() * c.GetFarmInfo().GetTimestamp()) // offline eggs
			contributionRatePerSecond += c.GetContributionRate()
		}
		startTime = startTime.Add(time.Duration(secondsRemaining) * time.Second)
		startTime = startTime.Add(-time.Duration(eiContract.Grade[grade].LengthInSeconds) * time.Second)
		totalReq := eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1]
		calcSecondsRemaining = int64((totalReq - totalContributions) / contributionRatePerSecond)
		endTime = nowTime.Add(time.Duration(calcSecondsRemaining) * time.Second)
		contractDurationSeconds = endTime.Sub(startTime).Seconds()
	}

	return startTime, contractDurationSeconds, err
}
