package boost

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/disgoorg/disgo/discord"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// commandDefinitions is every slash command definition main.go registers from
// this package, keyed by the constructor's name.
var commandDefinitions = map[string]func(string) *dc.Command{
	"GetSlashAdminContractsListCommand":  GetSlashAdminContractsListCommand,
	"GetSlashAvailabilityCommand":        GetSlashAvailabilityCommand,
	"GetSlashBoostCommand":               GetSlashBoostCommand,
	"GetSlashBoostOrderCommand":          GetSlashBoostOrderCommand,
	"GetSlashBumpCommand":                GetSlashBumpCommand,
	"GetSlashBumpCRCommand":              GetSlashBumpCRCommand,
	"GetSlashCalcContractTval":           GetSlashCalcContractTval,
	"GetSlashChangeCommand":              GetSlashChangeCommand,
	"GetSlashChangeOneBoosterCommand":    GetSlashChangeOneBoosterCommand,
	"GetSlashChangePlannedStartCommand":  GetSlashChangePlannedStartCommand,
	"GetSlashChangeSpeedRunSinkCommand":  GetSlashChangeSpeedRunSinkCommand,
	"GetSlashContractCommand":            GetSlashContractCommand,
	"GetSlashContractReportCommand":      GetSlashContractReportCommand,
	"GetSlashContractSettingsCommand":    GetSlashContractSettingsCommand,
	"GetSlashCoopETACommand":             GetSlashCoopETACommand,
	"GetSlashCoopTval":                   GetSlashCoopTval,
	"GetSlashCsEstimates":                GetSlashCsEstimates,
	"GetSlashEstimateTime":               GetSlashEstimateTime,
	"GetSlashHelpCommand":                GetSlashHelpCommand,
	"GetSlashJoinContractCommand":        GetSlashJoinContractCommand,
	"GetSlashLeaderboard":                GetSlashLeaderboard,
	"GetSlashLinkAlternateCommand":       GetSlashLinkAlternateCommand,
	"GetSlashLobbyCommand":               GetSlashLobbyCommand,
	"GetSlashPruneCommand":               GetSlashPruneCommand,
	"GetSlashRegisterAltCommand":         GetSlashRegisterAltCommand,
	"GetSlashRegisterCommand":            GetSlashRegisterCommand,
	"GetSlashRenameThread":               GetSlashRenameThread,
	"GetSlashRerunEvalCommand":           GetSlashRerunEvalCommand,
	"GetSlashScoreExplorerCommand":       GetSlashScoreExplorerCommand,
	"GetSlashSkipCommand":                GetSlashSkipCommand,
	"GetSlashSpeedrunCommand":            GetSlashSpeedrunCommand,
	"GetSlashStones":                     GetSlashStones,
	"GetSlashTeamworkEval":               GetSlashTeamworkEval,
	"GetSlashToggleContractPingsCommand": GetSlashToggleContractPingsCommand,
	"GetSlashTokenEditCommand":           GetSlashTokenEditCommand,
	"GetSlashUnboostCommand":             GetSlashUnboostCommand,
	"GetSlashUpdateCommand":              GetSlashUpdateCommand,
	"GetSlashUploadBannerCommand":        GetSlashUploadBannerCommand,
	"GetSlashVirtueCommand":              GetSlashVirtueCommand,
	"GetSlashVolunteerSink":              GetSlashVolunteerSink,
	"GetSlashVoluntellSink":              GetSlashVoluntellSink,
	"SlashAdminCurrentContracts":         SlashAdminCurrentContracts,
	"SlashAdminExitCommand":              SlashAdminExitCommand,
	"SlashAdminGetContractData":          SlashAdminGetContractData,
	"SlashAdminGuildStateCommand":        SlashAdminGuildStateCommand,
	"SlashAdminListRoles":                SlashAdminListRoles,
	"SlashAdminMembers":                  SlashAdminMembers,
	"SlashAdminStatusMessageCommand":     SlashAdminStatusMessageCommand,
	"SlashArtifactsCommand":              SlashArtifactsCommand,
}

// TestDumpCommandSchemas writes every command definition to the file named by
// BOOST_SCHEMA_DUMP. It is a tool, not an assertion: run it before and after a
// change to the definitions and diff the two files.
func TestDumpCommandSchemas(t *testing.T) {
	path := os.Getenv("BOOST_SCHEMA_DUMP")
	if path == "" {
		t.Skip("set BOOST_SCHEMA_DUMP to write a schema snapshot")
	}

	out := make(map[string]discord.ApplicationCommandCreate, len(commandDefinitions))
	for name, build := range commandDefinitions {
		out[name] = build("cmd-" + name).ToDisgo()
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
