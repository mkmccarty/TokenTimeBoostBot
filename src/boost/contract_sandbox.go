package boost

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

const srSandboxBaseURL = "https://srsandbox-staabmia.netlify.app/?"

// ContractSandboxOptions allows callers to override derived values when generating sandbox data from a Contract.
type ContractSandboxOptions struct {
	TargetEggOverride             *float64
	TokenTimerMinutesOverride     *int
	ContractLengthSecondsOverride *int
	CxpToggleOverride             *bool
}

func buildDefaultSandboxPlayers(count int) []SandboxPlayer {
	if count < 1 {
		count = 1
	}

	players := make([]SandboxPlayer, 0, count)
	for i := 0; i < count; i++ {
		players = append(players, SandboxPlayer{
			Name:         fmt.Sprintf("Player %d", i),
			Tokens:       "5",
			TE:           "50",
			Mirror:       false,
			Colleggtible: true,
			Sink:         i == count-1,
			Creator:      i == 0,
			Item1:        "00",
			Item2:        "00",
			Item3:        "00",
			Item4:        "00",
			Item5:        "00",
			Item6:        "00",
			Item7:        "00",
			Item8:        "00",
		})
	}

	return players
}

func getContractTargetEgg(contract *Contract, eiContract *ei.EggIncContract) (float64, error) {
	if eiContract == nil {
		return 0, errors.New("missing contract metadata")
	}

	if len(eiContract.TargetAmount) > 0 {
		return eiContract.TargetAmount[len(eiContract.TargetAmount)-1], nil
	}

	return 0, fmt.Errorf("no target amount for contract %s", contract.ContractID)
}

// GenerateContractSandboxData builds an SR Sandbox query string from a Contract and player data.
// It does not require coop status data.
func GenerateContractSandboxData(contract *Contract, players []SandboxPlayer) (string, error) {
	return generateContractSandboxDataWithOptions(contract, players, nil)
}

// GenerateContractSandboxDataWithOptions builds an SR Sandbox query string from a Contract and player data,
// allowing callers to override derived values (for example, grade-specific coop status data).
func GenerateContractSandboxDataWithOptions(contract *Contract, players []SandboxPlayer, options *ContractSandboxOptions) (string, error) {
	return generateContractSandboxDataWithOptions(contract, players, options)
}

func generateContractSandboxDataWithOptions(contract *Contract, players []SandboxPlayer, options *ContractSandboxOptions) (string, error) {
	if contract == nil {
		return "", errors.New("contract is nil")
	}
	if contract.ContractID == "" {
		return "", errors.New("contract ID is empty")
	}

	eiContract, ok := ei.EggIncContractsAll[contract.ContractID]
	if !ok {
		return "", fmt.Errorf("contract metadata not found for %s", contract.ContractID)
	}

	targetEgg := 0.0
	if options != nil && options.TargetEggOverride != nil {
		targetEgg = *options.TargetEggOverride
	} else {
		var err error
		targetEgg, err = getContractTargetEgg(contract, &eiContract)
		if err != nil {
			return "", err
		}
	}

	minutesPerToken := contract.MinutesPerToken
	if options != nil && options.TokenTimerMinutesOverride != nil {
		minutesPerToken = *options.TokenTimerMinutesOverride
	}
	if minutesPerToken <= 0 {
		minutesPerToken = eiContract.MinutesPerToken
	}
	if minutesPerToken <= 0 {
		minutesPerToken = 60
	}

	lengthInSeconds := contract.LengthInSeconds
	if options != nil && options.ContractLengthSecondsOverride != nil {
		lengthInSeconds = *options.ContractLengthSecondsOverride
	}
	if lengthInSeconds <= 0 {
		lengthInSeconds = eiContract.LengthInSeconds
	}
	if lengthInSeconds <= 0 {
		return "", fmt.Errorf("invalid contract duration for %s", contract.ContractID)
	}

	numPlayers := len(players)
	if numPlayers == 0 {
		numPlayers = contract.CoopSize
		players = buildDefaultSandboxPlayers(numPlayers)
		numPlayers = len(players)
	}

	cxpToggle := contract.SeasonalScoring == ei.SeasonalScoringNerfed
	if options != nil && options.CxpToggleOverride != nil {
		cxpToggle = *options.CxpToggleOverride
	}

	return EncodeSandboxData(
		cxpToggle,
		targetEgg,
		strconv.Itoa(minutesPerToken),
		lengthInSeconds,
		numPlayers,
		&eiContract,
		players,
	)
}

// GenerateContractSandboxURL builds the full SR Sandbox URL from a Contract and player data.
func GenerateContractSandboxURL(contract *Contract, players []SandboxPlayer) (string, error) {
	data, err := generateContractSandboxDataWithOptions(contract, players, nil)
	if err != nil {
		return "", err
	}
	return srSandboxBaseURL + data, nil
}

// BuildContractSandboxConfigWithOptions constructs a bottools.SRSandboxConfig from a Contract and player list.
func BuildContractSandboxConfigWithOptions(contract *Contract, players []SandboxPlayer, options *ContractSandboxOptions) (bottools.SRSandboxConfig, error) {
	if contract == nil {
		return bottools.SRSandboxConfig{}, errors.New("contract is nil")
	}
	if contract.ContractID == "" {
		return bottools.SRSandboxConfig{}, errors.New("contract ID is empty")
	}

	eiContract, ok := ei.EggIncContractsAll[contract.ContractID]
	if !ok {
		return bottools.SRSandboxConfig{}, fmt.Errorf("contract metadata not found for %s", contract.ContractID)
	}

	targetEgg := 0.0
	if options != nil && options.TargetEggOverride != nil {
		targetEgg = *options.TargetEggOverride
	} else {
		var err error
		targetEgg, err = getContractTargetEgg(contract, &eiContract)
		if err != nil {
			return bottools.SRSandboxConfig{}, err
		}
	}

	minutesPerToken := contract.MinutesPerToken
	if options != nil && options.TokenTimerMinutesOverride != nil {
		minutesPerToken = *options.TokenTimerMinutesOverride
	}
	if minutesPerToken <= 0 {
		minutesPerToken = eiContract.MinutesPerToken
	}
	if minutesPerToken <= 0 {
		minutesPerToken = 60
	}

	lengthInSeconds := contract.LengthInSeconds
	if options != nil && options.ContractLengthSecondsOverride != nil {
		lengthInSeconds = *options.ContractLengthSecondsOverride
	}
	if lengthInSeconds <= 0 {
		lengthInSeconds = eiContract.LengthInSeconds
	}
	if lengthInSeconds <= 0 {
		return bottools.SRSandboxConfig{}, fmt.Errorf("invalid contract duration for %s", contract.ContractID)
	}

	if len(players) == 0 {
		players = buildDefaultSandboxPlayers(contract.CoopSize)
	}

	cxpToggle := contract.SeasonalScoring == ei.SeasonalScoringNerfed
	if options != nil && options.CxpToggleOverride != nil {
		cxpToggle = *options.CxpToggleOverride
	}

	ggMultiplier, ultraGGMultiplier, _ := ei.GetGenerousGiftEvent()
	ggToggle := ggMultiplier > 1.0 || ultraGGMultiplier > 1.0

	modStr, modIdx := FmtActiveModifier(&eiContract)
	modMult, err := strconv.ParseFloat(modStr, 64)
	if err != nil || modMult <= 0 {
		modMult = 1.0
	}

	btvTarget := 2.0
	if cxpToggle {
		btvTarget = 1.875
	}

	contractName := contract.Name
	if contractName == "" {
		contractName = eiContract.Name
	}
	if contractName == "" {
		contractName = contract.ContractID
	}

	simPlayers := make([]bottools.SRSandboxPlayerInput, 0, len(players))
	for _, p := range players {
		simPlayers = append(simPlayers, bottools.NewSRSandboxPlayerInputFromCodes(
			p.Name,
			p.Tokens,
			p.TE,
			p.Mirror,
			p.Colleggtible,
			p.Sink,
			p.Creator,
			p.Item1,
			p.Item2,
			p.Item3,
			p.Item4,
			p.Item5,
			p.Item6,
			p.Item7,
			p.Item8,
		))
	}

	return bottools.SRSandboxConfig{
		ContractName:      contractName,
		CRTRun:            true,
		TokenToggle:       true,
		GG:                ggToggle,
		EarlySwap:         false,
		CxpToggle:         cxpToggle,
		JoinDelaySeconds:  20,
		MinPerTokenGift:   13,
		DurationSeconds:   float64(lengthInSeconds),
		TargetEggAmount:   targetEgg,
		TokenTimerMinutes: float64(minutesPerToken),
		ModifierType:      bottools.SRSandboxModifierType(modIdx),
		ModifierMult:      modMult,
		BTVTarget:         btvTarget,
		Players:           simPlayers,
	}, nil
}

// GenerateContractSandboxResults computes the SR Sandbox simulation results and renders the Results image table PNG.
func GenerateContractSandboxResults(contract *Contract, players []SandboxPlayer, options *ContractSandboxOptions) (bottools.SRSandboxResults, bottools.SRSandboxTableData, []byte, error) {
	cfg, err := BuildContractSandboxConfigWithOptions(contract, players, options)
	if err != nil {
		return bottools.SRSandboxResults{}, bottools.SRSandboxTableData{}, nil, err
	}
	return bottools.GenerateSRSandboxResultsImage(cfg)
}

// BuildSandboxMessage builds the SR Sandbox message payload for a contract, including the link and Results image table.
func BuildSandboxMessage(contract *Contract) (dc.Message, error) {
	players := sandboxPlayersFromContract(contract)
	sandboxURL, err := GenerateContractSandboxURL(contract, players)
	if err != nil {
		return dc.Message{}, err
	}

	contractName := contract.Name
	if contractName == "" {
		if contractInfo, ok := ei.EggIncContractsAll[contract.ContractID]; ok {
			contractName = contractInfo.Name
		}
	}
	if contractName == "" {
		contractName = contract.ContractID
	}

	var channelMention string
	var contractLink string
	if len(contract.Location) > 0 && contract.Location[0] != nil {
		contractLink = fmt.Sprintf("https://discord.com/channels/%s/%s/%s", contract.Location[0].GuildID, contract.Location[0].ChannelID, contract.Location[0].ListMsgID)
		channelMention = contract.Location[0].ChannelMention
	}

	var body string
	if contractLink != "" {
		body = fmt.Sprintf(
			"SR Sandbox for %s\nCoop: %s\nSignup: %d/%d\nChannel: %s\n[Open contract thread](%s)\n[Open SR Sandbox](%s)",
			contractName,
			contract.CoopID,
			len(contract.Boosters),
			contract.CoopSize,
			channelMention,
			contractLink,
			sandboxURL,
		)
	} else {
		body = fmt.Sprintf(
			"SR Sandbox for %s\nCoop: %s\nSignup: %d/%d\n[Open SR Sandbox](%s)",
			contractName,
			contract.CoopID,
			len(contract.Boosters),
			contract.CoopSize,
			sandboxURL,
		)
	}

	var files []dc.File
	if len(body) > 2000 {
		if contractLink != "" {
			body = fmt.Sprintf(
				"SR Sandbox for %s\nCoop: %s\nSignup: %d/%d\nChannel: %s\n[Open contract thread](%s)\nOpen SR Sandbox: URL is attached as a text file.",
				contractName,
				contract.CoopID,
				len(contract.Boosters),
				contract.CoopSize,
				channelMention,
				contractLink,
			)
		} else {
			body = fmt.Sprintf(
				"SR Sandbox for %s\nCoop: %s\nSignup: %d/%d\nOpen SR Sandbox: URL is attached as a text file.",
				contractName,
				contract.CoopID,
				len(contract.Boosters),
				contract.CoopSize,
			)
		}
		files = append(files, dc.File{
			Name:        "sandbox_url.txt",
			ContentType: "text/plain",
			Reader:      strings.NewReader(sandboxURL),
		})
	}

	if _, _, imgBytes, imgErr := GenerateContractSandboxResults(contract, players, nil); imgErr == nil && len(imgBytes) > 0 {
		files = append(files, dc.File{
			Name:        "sr_sandbox_results.png",
			ContentType: "image/png",
			Reader:      bytes.NewReader(imgBytes),
		})
	}

	return dc.Message{
		Content: body,
		Files:   files,
	}, nil
}

// SendSandboxDM builds and sends an SR Sandbox DM to a user.
func SendSandboxDM(client dc.Client, contract *Contract, userID string) error {
	msg, err := BuildSandboxMessage(contract)
	if err != nil {
		return err
	}

	u, dmErr := client.CreateUserChannel(userID)
	if dmErr != nil {
		return dmErr
	}

	msg.ComponentsV1 = true
	msg.Components = []dc.LayoutComponent{
		dc.ActionRow{
			Components: []dc.InteractiveComponent{
				dc.Button{
					Label:    "Dismiss",
					Style:    dc.ButtonSecondary,
					CustomID: "rc_#dismiss#" + contract.ContractHash,
				},
			},
		},
	}

	_, sendErr := client.SendMessage(u.ID, msg)
	return sendErr
}

// GenerateContractSandboxURLWithOptions builds the full SR Sandbox URL from a Contract and player data,
// allowing callers to override derived values (for example, grade-specific coop status data).
func GenerateContractSandboxURLWithOptions(contract *Contract, players []SandboxPlayer, options *ContractSandboxOptions) (string, error) {
	data, err := generateContractSandboxDataWithOptions(contract, players, options)
	if err != nil {
		return "", err
	}
	return srSandboxBaseURL + data, nil
}
