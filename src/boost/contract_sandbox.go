package boost

import (
	"errors"
	"fmt"
	"strconv"

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

// GenerateContractSandboxURLWithOptions builds the full SR Sandbox URL from a Contract and player data,
// allowing callers to override derived values (for example, grade-specific coop status data).
func GenerateContractSandboxURLWithOptions(contract *Contract, players []SandboxPlayer, options *ContractSandboxOptions) (string, error) {
	data, err := generateContractSandboxDataWithOptions(contract, players, options)
	if err != nil {
		return "", err
	}
	return srSandboxBaseURL + data, nil
}
