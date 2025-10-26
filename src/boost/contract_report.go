package boost

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

const (
	nameW  = 7
	cxpW   = 6
	contrW = 5
	teamW  = 5
	crW    = 2
	btvW   = 6
	deltaW = 7

	maxParallel = 20 // max concurrent EI API fetches
)

// ErrNoChannelContract is returned when no contract can be found for the specified channel.
// ErrEvaluationNotFound is returned when an expected evaluation for a contract cannot be found.
// ErrCoopIDMissing indicates that a required coop identifier was not provided.
// ErrUnsupportedCXPVersion indicates that the provided CXP protocol or API version is not supported.
// ErrCoopStatusFetch is returned when fetching the coop status from an external source fails.
// ErrCoopStatusResponse indicates that the coop status endpoint returned an error or invalid response.
// ErrCoopNoGrade is returned when no grade is available for the coop.
// ErrCoopNotFinished indicates that the coop has not yet reached a finished state.
// ErrContribProcess is returned when processing contributor information fails.
// ErrInvalidContractID indicates that the provided contract identifier is invalid or malformed.
// ErrCoopGradeInvalid is returned when the coop grade index is out of range or otherwise invalid.
// ErrReportSendFailed indicates that sending or publishing the report failed.
// ErrContractDurationMismatch is returned when the actual contract duration does not match the expected duration.
var (
	ErrNoChannelContract        = errors.New("no contract found for channel")
	ErrEvaluationNotFound       = errors.New("evaluation not found for contract")
	ErrCoopIDMissing            = errors.New("coop id missing")
	ErrUnsupportedCXPVersion    = errors.New("unsupported cxp version")
	ErrCoopStatusFetch          = errors.New("coop status fetch failed")
	ErrCoopStatusResponse       = errors.New("coop status error")
	ErrCoopNoGrade              = errors.New("coop grade missing")
	ErrCoopNotFinished          = errors.New("coop not finished")
	ErrContribProcess           = errors.New("contributors processing failed")
	ErrInvalidContractID        = errors.New("invalid contract id")
	ErrCoopGradeInvalid         = errors.New("invalid coop grade index")
	ErrReportSendFailed         = errors.New("report send failed")
	ErrContractDurationMismatch = errors.New("contract duration mismatch")
)

func userMessage(err error) string {
	switch {
	case errors.Is(err, ErrNoChannelContract):
		return "No contract found in this channel. Please provide a contract-id."
	case errors.Is(err, ErrEvaluationNotFound):
		return "Evaluation not found, if you just completed the contract please wait a few minutes and try again with refresh=true."
	case errors.Is(err, ErrCoopIDMissing):
		return "No coop ID found for this contract evaluation."
	case errors.Is(err, ErrUnsupportedCXPVersion):
		return "Unsupported CXP version for this contract (need 0.2.0)."
	case errors.Is(err, ErrCoopStatusFetch):
		return "Failed to fetch coop status."
	case errors.Is(err, ErrCoopStatusResponse):
		return "Coop status returned an error."
	case errors.Is(err, ErrCoopNoGrade):
		return "No grade found for this contract."
	case errors.Is(err, ErrCoopNotFinished):
		return "This coop hasn’t finished the contract yet."
	case errors.Is(err, ErrContribProcess):
		return "Failed to process coop participants."
	case errors.Is(err, ErrInvalidContractID):
		return "Invalid contract ID."
	case errors.Is(err, ErrCoopGradeInvalid):
		return "Invalid coop grade for this contract."
	case errors.Is(err, ErrReportSendFailed):
		return "I couldn't send the report. Please try again."
	case errors.Is(err, ErrContractDurationMismatch):
		return "The contract duration from the evaluation does not match the coop status"
	default:
		return "Something went wrong while building the report."
	}
}

type contractReportParameters struct {
	contractID         string
	coopID             string
	startTime          time.Time
	endTime            time.Time
	contractDur        time.Duration
	missingPlayers     []string
	thresholds         thresholds
	playerEvalsMetrics []evalMetrics
	metricPeaks        metricPeaks
	contract           *ei.EggIncContract
}

type thresholds struct {
	chickenRuns   float64 // e.g. 20
	buffTimeValue float64 // e.g. dur * 2.0
	teamwork      float64 // e.g. 26.0 / 19.0
	deltaTVal     float64 // e.g. 3.0
}

type evalMetrics struct {
	player            string
	cxp               float64
	contributionRatio float64
	teamwork          float64
	chickenRunsSent   uint32
	buffTimeValue     float64
	deltaTVal         float64 // sent - received
}

type metricPeaks struct {
	cxp               float64
	teamwork          float64
	contributionRatio float64
	buffTimeValue     float64
}

// GetSlashContractReportCommand returns the command for the /contract-report command
func GetSlashContractReportCommand(cmd string) *discordgo.ApplicationCommand {
	//minValue := 0.0
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Generate contract report from player EIs.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Select a contract-id",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "refresh",
				Description: "If you want to force a refresh due a recent change to your contracts.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "missing-players",
				Description: "Show missing players in the report. Default is false.",
				Required:    false,
			},
		},
	}
}

// HandleContractReport handles the /contract-report command
func HandleContractReport(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	optionMap := bottools.GetCommandOptionsMap(i)
	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}
	if err := ContractReport(s, i, optionMap, userID, true); err != nil {
		log.Printf("ContractReport failed: %v", err)

		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags: discordgo.MessageFlagsIsComponentsV2,
			Components: []discordgo.MessageComponent{
				&discordgo.TextDisplay{Content: userMessage(err)},
			},
		})
		if err != nil {
			log.Println("Error sending error message:", err)
		}
		return
	}

}

func processContributors(
	s *discordgo.Session,
	coopStatus *ei.ContractCoopStatusResponse,
	callerUserID, cxpVersion string,
	forceRefresh, okayToSave bool,
) (map[string][]*ei.LocalContract, []string, error) {
	if coopStatus == nil {
		return nil, nil, errors.New("coopStatus is nil")
	}

	encKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("decode enc key: %w", err)
	}

	contribs := coopStatus.GetContributors()
	n := len(contribs)
	if n == 0 {
		return make(map[string][]*ei.LocalContract), nil, nil
	}

	maxParallel := max(min(n, maxParallel), 1)

	namesCh := make(chan string, n)
	var wg sync.WaitGroup

	evalsByName := make(map[string][]*ei.LocalContract, n)
	var (
		muEval   sync.Mutex
		muMiss   sync.Mutex
		missing  = make([]string, 0, n)
		firstErr error
		errOnce  sync.Once
	)

	worker := func() {
		defer wg.Done()
		for name := range namesCh {
			discordID, _ := farmerstate.GetDiscordUserIDFromEiIgn(name)
			if discordID == "" || discordID == callerUserID {
				if discordID == "" {
					muMiss.Lock()
					missing = append(missing, name)
					muMiss.Unlock()
				}
				continue
			}

			eiCipherB64 := farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
			if eiCipherB64 == "" {
				muMiss.Lock()
				missing = append(missing, name)
				muMiss.Unlock()
				continue
			}
			cipherBytes, err := base64.StdEncoding.DecodeString(eiCipherB64)
			if err != nil {
				muMiss.Lock()
				missing = append(missing, name)
				muMiss.Unlock()
				continue
			}
			plain, err := config.DecryptCombined(encKey, cipherBytes)
			if err != nil {
				muMiss.Lock()
				missing = append(missing, name)
				muMiss.Unlock()
				continue
			}
			eiID := string(plain)
			if len(eiID) != 18 || !strings.HasPrefix(eiID, "EI") {
				muMiss.Lock()
				missing = append(missing, name)
				muMiss.Unlock()
				continue
			}

			// cache IGN if missing
			if ign := farmerstate.GetMiscSettingString(discordID, "ei_ign"); ign == "" {
				if backup, _ := ei.GetFirstContactFromAPI(s, eiID, discordID, okayToSave); backup != nil {
					farmerstate.SetMiscSettingString(discordID, "ei_ign", backup.GetUserName())
				}
			}

			archive, cached := ei.GetContractArchiveFromAPI(s, eiID, discordID, forceRefresh, okayToSave)

			// record archive by contributor name
			muEval.Lock()
			evalsByName[name] = archive
			muEval.Unlock()

			// write per-contributor cache file
			if !cached && okayToSave {
				jsonData, err := json.Marshal(archive)
				if err != nil {
					errOnce.Do(func() { firstErr = fmt.Errorf("marshal archive: %w", err) })
					continue
				}
				jsonData = bytes.ReplaceAll(jsonData, []byte(eiID), []byte(discordID))

				fileName := fmt.Sprintf("ttbb-data/eiuserdata/archive-%s-%s.json", discordID, cxpVersion)
				if err := os.WriteFile(fileName, jsonData, 0o644); err != nil {
					errOnce.Do(func() { firstErr = fmt.Errorf("write archive file: %w", err) })
				}
			}
		}
	}

	wg.Add(maxParallel)
	for range maxParallel {
		go worker()
	}
	// enqueue contributors
	for _, c := range contribs {
		namesCh <- c.GetUserName()
	}
	close(namesCh)
	wg.Wait()

	return evalsByName, missing, firstErr
}

// ContractReport generates a contract report for player's contract with the given contract ID
//
// Parameters:
//   - s: active Discord session
//   - i: the triggering interaction
//   - optionMap: slash-command options (e.g., "contract-id", "refresh").
//   - userID: Discord user ID of the caller
//   - okayToSave: whether API fetches may be cached/persisted.
//
// Returns:
//   - error: nil on success.
func ContractReport(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption,
	userID string,
	okayToSave bool,
) error {

	// define parameter struct
	p := contractReportParameters{}

	// --- Options ---
	forceRefresh := false
	if opt, ok := optionMap["refresh"]; ok {
		forceRefresh = opt.BoolValue()
	}
	showMissingPlayers := false
	if opt, ok := optionMap["missing-players"]; ok {
		showMissingPlayers = opt.BoolValue()
	}

	// resolve contractID, prefer the slash option.
	var contractID string
	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.ToLower(opt.StringValue())
		contractID = strings.ReplaceAll(contractID, " ", "")
	}
	// If absent, fall back to the channel's contract.
	if contractID == "" {
		c := FindContract(i.ChannelID)
		if c != nil && c.ContractID != "" {
			contractID = strings.ToLower(c.ContractID)
			contractID = strings.ReplaceAll(contractID, " ", "")
		} else {
			return ErrNoChannelContract
		}
	}

	// decrypt user's EI
	callerEncryptedEI := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	callerEI := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		callerDecodedData, err := base64.StdEncoding.DecodeString(callerEncryptedEI)
		if err == nil {
			callerDecryptedData, err := config.DecryptCombined(encryptionKey, callerDecodedData)
			if err == nil {
				callerEI = string(callerDecryptedData)
			}
		}
	}
	// validate EI or prompt
	if len(callerEI) != 18 || !strings.HasPrefix(callerEI, "EI") {
		RequestEggIncIDModal(s, i, "contract-report", optionMap)
		return nil
	}

	// Quick reply to buy us some time
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	callerUserID := bottools.GetInteractionUserID(i)
	// Do I know the user's IGN?
	callerFarmerName := farmerstate.GetMiscSettingString(callerUserID, "ei_ign")
	if callerFarmerName == "" {
		backup, _ := ei.GetFirstContactFromAPI(s, callerEI, callerUserID, okayToSave)
		if backup != nil {
			// save
			callerFarmerName = backup.GetUserName()
			farmerstate.SetMiscSettingString(callerUserID, "ei_ign", callerFarmerName)
		}
	}
	callerArchive, callerCached := ei.GetContractArchiveFromAPI(s, callerEI, callerUserID, forceRefresh, okayToSave)

	// Locate the caller’s evaluation for this specific contract, then validate it.
	cxpVersion := ""
	var callerEval *ei.ContractEvaluation
	for _, lc := range callerArchive {
		// Check if we have an evaluation with the given contract ID
		if c := lc.GetContract(); c != nil && c.GetIdentifier() == contractID {
			if eval := lc.GetEvaluation(); eval != nil {
				// We found the contract, check the evaluation version
				callerEval = eval
				v := eval.GetVersion()
				// Replace all non-numeric characters in cxpVersion with underscores
				cxpVersion = strings.Map(func(r rune) rune {
					if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
						return r
					}
					return '_'
				}, v)
			}
			break
		}
	}
	var coopID string
	if callerEval == nil {
		return ErrEvaluationNotFound
	} else if cxpVersion != "cxp_v0_2_0" {
		return ErrUnsupportedCXPVersion
	} else if coopID = callerEval.GetCoopIdentifier(); coopID == "" {
		return ErrCoopIDMissing
	}

	// Get coop status; validate response
	// Using GetCoopStatusForCompletedContracts to ensure we cache completed contract data locally
	coopStatus, nowTime, _, err := ei.GetCoopStatusForCompletedContracts(contractID, coopID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCoopStatusFetch, err)
	}
	rs := coopStatus.GetResponseStatus()
	if rs != ei.ContractCoopStatusResponse_NO_ERROR {
		return fmt.Errorf("%w: %s", ErrCoopStatusResponse,
			ei.ContractCoopStatusResponse_ResponseStatus_name[int32(rs)])
	}
	if coopStatus.GetGrade() == ei.Contract_GRADE_UNSET {
		return ErrCoopNoGrade
	}
	if coopStatus.GetSecondsSinceAllGoalsAchieved() <= 0 {
		return ErrCoopNotFinished
	}

	evalsByName, missing, perr := processContributors(
		s,          // *discordgo.Session
		coopStatus, // *ei.ContractCoopStatusResponse
		callerUserID,
		cxpVersion,
		forceRefresh, okayToSave,
	)
	if perr != nil {
		return fmt.Errorf("%w: %v", ErrContribProcess, perr)
	}
	if len(missing) > 0 {
		log.Println("Contributors missing Discord/EI:", strings.Join(missing, ", "))
	}
	evByName := evalsForContractParallel(evalsByName, contractID)

	// contract lookup
	c := ei.EggIncContractsAll[contractID]
	if c.ID == "" {
		return ErrInvalidContractID
	}

	// derive times from coop status (finished coop guaranteed earlier)
	gradeIdx := int(coopStatus.GetGrade())
	if gradeIdx < 0 || gradeIdx >= len(c.Grade) {
		return ErrCoopGradeInvalid
	}

	startTime := nowTime
	endTime := nowTime
	lengthSec := float64(c.Grade[gradeIdx].LengthInSeconds)

	// start = now + secondsRemaining - contractLength
	startTime = startTime.Add(time.Duration(coopStatus.GetSecondsRemaining()) * time.Second)
	startTime = startTime.Add(-time.Duration(lengthSec) * time.Second)

	// end = now - secondsSinceAllGoalsAchieved
	endTime = endTime.Add(-time.Duration(coopStatus.GetSecondsSinceAllGoalsAchieved()) * time.Second)

	// duration from evaluation
	contractDur := time.Duration(math.Round(callerEval.GetCompletionTime() * float64(time.Second)))

	// Check if the duration is within error of endTime - startTime
	calculatedDur := endTime.Sub(startTime)
	if math.Abs(contractDur.Seconds()-calculatedDur.Seconds()) > 10.0 {
		return ErrContractDurationMismatch
	}

	// set parameters
	p.contractDur = contractDur
	p.contract = &c
	p.thresholds = deriveThresholds(&p) // MUST be after p.contract and p.contractDur are set
	p.contractID = contractID
	p.coopID = coopID
	p.startTime = startTime
	p.endTime = endTime
	p.missingPlayers = missing
	p.playerEvalsMetrics, p.metricPeaks = buildAndSortEvals(callerFarmerName, callerEval, evByName)

	// render components
	components := printContractReport(&p, showMissingPlayers)
	if len(components) == 0 {
		components = []discordgo.MessageComponent{
			&discordgo.TextDisplay{Content: "No archived contracts found in Egg Inc API response"},
		}
	}
	if _, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrReportSendFailed, err)
	}

	// cache (best-effort; non-fatal)
	if !callerCached && okayToSave {
		if config.IsDevBot() {
			go func() {
				jsonData, merr := json.Marshal(callerArchive)
				if merr != nil {
					log.Println("Error marshalling archive to JSON:", merr)
				} else {
					fileName := fmt.Sprintf("ttbb-data/eiuserdata/archive-%s-%s.json", callerUserID, cxpVersion)
					jsonData = bytes.ReplaceAll(jsonData, []byte(callerEI), []byte(callerUserID))
					if werr := os.WriteFile(fileName, jsonData, 0o644); werr != nil {
						log.Println("Error saving contract archive to file:", werr)
					}
				}
			}()
		}
	}
	return nil
}

// printContractReport returns two components:
//  1. markdown header with thresholds
//  2. the ANSI table (with an em-dash rule between header and rows)
//     If the contract is seasonal-nerfed, the ΔTVal column **and** its threshold are omitted.
func printContractReport(
	p *contractReportParameters,
	showMissingPlayers bool,
) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent

	currentContract := p.contract
	nerfed := currentContract != nil && currentContract.SeasonalScoring == ei.SeasonalScoringNerfed

	// --- Header (markdown) ---
	var h strings.Builder
	// Round/format thresholds for display
	btvStr := fmt.Sprintf("%d", int(math.Round(p.thresholds.buffTimeValue)))
	crStr := fmt.Sprintf("%g", p.thresholds.chickenRuns)
	dtvStr := fmt.Sprintf("%.2f", p.thresholds.deltaTVal)

	// Build contract info strings
	seasonalStr := ""
	if currentContract.SeasonID != "" {
		parts := strings.Split(currentContract.SeasonID, "_")
		if len(parts) >= 2 {
			seasonIcon, seasonYear := parts[0], parts[1]
			seasonEmote := map[string]string{"winter": "❄️", "spring": "🌷", "summer": "☀️", "fall": "🍂"}
			seasonalStr = fmt.Sprintf("Seasonal: %s %s\n", seasonEmote[seasonIcon], seasonYear)
		}
	}

	h.WriteString(fmt.Sprintf("%s **%s** `%s` %s\n%sCode: [%s](%s) - %s %d - 📏 %s **/** %s\n",
		FindEggEmoji(currentContract.EggName), currentContract.Name, currentContract.ID, ei.GetBotEmojiMarkdown("contract_grade_aaa"), seasonalStr,
		p.coopID, fmt.Sprintf("https://eicoop-carpet.netlify.app/%s/%s", p.contractID, p.coopID),
		ei.GetBotEmojiMarkdown("icon_coop"), currentContract.MaxCoopSize,
		bottools.FmtDuration(p.contractDur), bottools.FmtDuration(time.Duration(currentContract.LengthInSeconds)*time.Second),
	))
	h.WriteString(fmt.Sprintf("Start Time: <t:%d:f>\nEnd Time:   <t:%d:f>\n", p.startTime.Unix(), p.endTime.Unix()))

	// Threshold line (omit ΔTVal when nerfed)
	if nerfed {
		h.WriteString(fmt.Sprintf("🎯 Thresholds: `%s` BTV, `%s` CRs\n\n", btvStr, crStr))
	} else {
		h.WriteString(fmt.Sprintf("🎯 Thresholds: `%s` BTV, `%s` CRs, `%s` ΔTVal\n\n", btvStr, crStr, dtvStr))
	}

	if len(p.missingPlayers) > 0 {
		h.WriteString(fmt.Sprintf("__Members__ (%d out of %d players matched)\n", len(p.playerEvalsMetrics), currentContract.MaxCoopSize))
	} else {
		h.WriteString(fmt.Sprintf("__Members__ (%d players)\n", len(p.playerEvalsMetrics)))
	}
	components = append(components, &discordgo.TextDisplay{Content: h.String()})

	// --- ANSI Table ---
	if len(p.playerEvalsMetrics) > 0 {
		var b strings.Builder
		b.WriteString("```ansi\n")

		header := evalMetricsHeader(nerfed)
		b.WriteString(header)
		b.WriteByte('\n')

		// em-dash rule between header and rows
		b.WriteString(strings.Repeat("—", bottools.VisibleLenANSI(header)))
		b.WriteByte('\n')

		for _, e := range p.playerEvalsMetrics {
			b.WriteString(formatEvalMetricsRowANSI(
				e.player, e.cxp, e.contributionRatio, e.teamwork, e.chickenRunsSent, e.buffTimeValue, e.deltaTVal,
				p.thresholds, p.metricPeaks, nerfed,
			))
			b.WriteByte('\n')
		}
		b.WriteString("```")

		components = append(components, &discordgo.TextDisplay{Content: b.String()})
	}
	if len(p.missingPlayers) > 0 {
		var b strings.Builder
		// message about registration
		registerMessage := fmt.Sprintf("Missing from the report? Register your Egg Inc ID with %s.\n", bottools.GetFormattedCommand("register"))
		if showMissingPlayers {
			b.WriteString("Boost Bot doesn't know these players:\n")
			// sort missing player names
			names := append([]string(nil), p.missingPlayers...)
			slices.SortFunc(names, func(a, b string) int {
				if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
					return c
				}
				return strings.Compare(a, b)
			})

			// build missing players list, account for `-` in names (thx -wittysquid-)

			for i, s := range names {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteByte('`')
				b.WriteString(strings.ReplaceAll(s, "-", "\u2011"))
				b.WriteByte('`')
			}

		}
		components = append(components, &discordgo.TextDisplay{
			Content: registerMessage + b.String(),
		})
	}
	return components
}

// ===== header & row formatting =====

// Player | Cxp | Contr | Tmwk | CR | BTV | [ΔTVal*]
// If nerfed==true, omit ΔTVal.
func evalMetricsHeader(nerfed bool) string {
	cells := []string{
		bottools.AlignString("Player", nameW, bottools.StringAlignLeft),
		bottools.AlignString("Cxp", cxpW, bottools.StringAlignCenterRight),
		bottools.AlignString("Contr", contrW, bottools.StringAlignCenterRight),
		bottools.AlignString("TmWk", teamW, bottools.StringAlignCenterRight),
		bottools.AlignString("CR", crW, bottools.StringAlignCenterRight),
		bottools.AlignString("BTV", btvW, bottools.StringAlignCenterRight),
	}
	if !nerfed {
		cells = append(cells, bottools.AlignString("ΔTVal", deltaW, bottools.StringAlignRight))
	}
	return strings.Join(cells, "|")
}

// If nerfed==true, omits the ΔTVal cell.
func formatEvalMetricsRowANSI(
	player string,
	cxp float64,
	contr float64,
	teamwork float64,
	cr uint32,
	btv float64,
	dtval float64,
	th thresholds,
	peaks metricPeaks,
	nerfed bool,
) string {
	// base colors
	cxpBase := ""
	teamBase := colorIfGE(teamwork, th.teamwork, "green")
	contrBase := ""
	crBase := colorIfGE(float64(cr), th.chickenRuns, "green")
	btvBase := colorIfGE(btv, th.buffTimeValue, "green")
	dtColor := dtvalColor(dtval, th.deltaTVal)

	// peak override -> blue
	cxpColor := peakColor(cxp, peaks.cxp, cxpBase, true)
	teamColor := peakColor(teamwork, peaks.teamwork, teamBase, false)
	contrColor := peakColor(contr, peaks.contributionRatio, contrBase, true)
	btvColor := peakColor(btv, peaks.buffTimeValue, btvBase, true)

	cells := []string{
		fitName(player, nameW),
		bottools.CellANSI(fmt.Sprintf("%d", int(cxp)), cxpColor, cxpW, true),
		bottools.CellANSI(fmt.Sprintf("%.3f", contr), contrColor, contrW, true),
		bottools.CellANSI(fmt.Sprintf("%.3f", teamwork), teamColor, teamW, true),
		bottools.CellANSI(fmt.Sprintf("%d", cr), crBase, crW, true),
		bottools.CellANSI(fmt.Sprintf("%.0f", btv), btvColor, btvW, true),
	}
	if !nerfed {
		cells = append(cells,
			bottools.CellANSI(fmt.Sprintf("%.3f", dtval), dtColor, deltaW, true),
		)
	}
	return strings.Join(cells, "|")
}

// pad/truncate a plain name to width
func fitName(name string, width int) string {
	rs := []rune(name)
	if len(rs) > width {
		rs = rs[:width]
	}
	return fmt.Sprintf("%-*s", width, string(rs))
}

// ===== color rules =====

// return color if v >= th, else ""
func colorIfGE(v, th float64, color string) string {
	if v >= th {
		return color
	}
	return ""
}

// ΔTVal: red if < 0, green if >= threshold, else ""
func dtvalColor(v, th float64) string {
	if v < 0 {
		return "red"
	}
	if v >= th {
		return "green"
	}
	return ""
}

func approxEqual(a, b float64) bool {
	diff := math.Abs(a - b)
	scale := math.Max(math.Abs(a), math.Abs(b))
	tol := math.Max(1e-6, 1e-3*scale) // 0.1% or 1e-6 minimum
	return diff <= tol
}

// if v is the peak -> blue; otherwise keep baseColor
func peakColor(v, peak float64, baseColor string, exact bool) string {
	if exact {
		if v == peak {
			return "blue"
		}
	} else {
		if approxEqual(v, peak) {
			return "blue"
		}
	}
	return baseColor
}

// ===== data & selection =====

func deriveThresholds(p *contractReportParameters) thresholds {

	durationSec := p.contractDur.Seconds()
	contract := p.contract
	seasonalScoring := contract.SeasonalScoring

	// teamwork fixed for now since theoratical theamwork max can't be achieved in practice
	th := thresholds{
		teamwork: 26.0 / 19.0,
	}
	th.chickenRuns = GetTargetChickenRun(seasonalScoring, contract.MaxCoopSize, float64(contract.LengthInSeconds))
	th.buffTimeValue = GetTargetBuffTimeValue(seasonalScoring, durationSec)
	th.deltaTVal = GetTargetTval(seasonalScoring, durationSec/60., float64(contract.MinutesPerToken))

	return th
}

// pick the evaluation for a specific contractID from an archive
func evalForContract(archive []*ei.LocalContract, contractID string) *ei.ContractEvaluation {
	for _, lc := range archive {
		if c := lc.GetContract(); c != nil && c.GetIdentifier() == contractID {
			return lc.GetEvaluation()
		}
	}
	return nil
}

// Scan each player's archive concurrently, return only the matching evaluation.
func evalsForContractParallel(
	evalsByName map[string][]*ei.LocalContract,
	contractID string,
) map[string]*ei.ContractEvaluation {
	type job struct {
		name string
		arch []*ei.LocalContract
	}

	// determine number of workers
	N := max(min(len(evalsByName), runtime.NumCPU()), 1)

	jobs := make(chan job)
	var wg sync.WaitGroup
	out := make(map[string]*ei.ContractEvaluation, len(evalsByName))
	var mu sync.Mutex

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			if ev := evalForContract(j.arch, contractID); ev != nil {
				mu.Lock()
				out[j.name] = ev
				mu.Unlock()
			}
		}
	}

	wg.Add(N)
	for range N {
		go worker()
	}
	for name, arch := range evalsByName {
		jobs <- job{name: name, arch: arch}
	}
	close(jobs)
	wg.Wait()
	return out
}

// extract evalMetrics from ContractEvaluation
func metricsFromEval(name string, ev *ei.ContractEvaluation) evalMetrics {
	return evalMetrics{
		player:            name,
		cxp:               ev.GetCxp(),
		contributionRatio: ev.GetContributionRatio(),
		teamwork:          ev.GetTeamworkScore(),
		chickenRunsSent:   ev.GetChickenRunsSent(),
		buffTimeValue:     ev.GetBuffTimeValue(),
		deltaTVal:         ev.GetGiftTokenValueSent() - ev.GetGiftTokenValueReceived(),
	}
}

// build evalMetrics slice and compute peaks
func buildAndSortEvals(
	callerName string,
	callerEval *ei.ContractEvaluation,
	evByName map[string]*ei.ContractEvaluation,
) ([]evalMetrics, metricPeaks) {
	out := make([]evalMetrics, 0, 1+len(evByName))

	peaks := metricPeaks{
		cxp:               math.Inf(-1),
		teamwork:          math.Inf(-1),
		contributionRatio: math.Inf(-1),
		buffTimeValue:     math.Inf(-1),
	}

	add := func(name string, ev *ei.ContractEvaluation) {
		if ev == nil {
			return
		}
		m := metricsFromEval(name, ev)
		out = append(out, m)
		if m.cxp > peaks.cxp {
			peaks.cxp = m.cxp
		}
		if m.teamwork > peaks.teamwork {
			peaks.teamwork = m.teamwork
		}
		if m.contributionRatio > peaks.contributionRatio {
			peaks.contributionRatio = m.contributionRatio
		}
		if m.buffTimeValue > peaks.buffTimeValue {
			peaks.buffTimeValue = m.buffTimeValue
		}
	}

	add(callerName, callerEval)
	for name, ev := range evByName {
		add(name, ev)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].cxp > out[j].cxp })

	if len(out) == 0 {
		peaks = metricPeaks{}
	}

	return out, peaks
}

/*
{
	"evaluation": {
		"contract_identifier": "birthday-cake-2023",
		"coop_identifier": "happy-token",
		"cxp": 24702,
		"old_league": 0,
		"grade": 0,
		"contribution_ratio": 5.815095492301126,
		"completion_percent": 1,
		"original_length": 432000,
		"coop_size": 10,
		"solo": false,
		"soul_power": 30.02439174202951,
		"last_contribution_time": 1680055626.437586,
		"completion_time": 91932.26965808868,
		"chicken_runs_sent": 5,
		"gift_tokens_sent": 7,
		"gift_tokens_received": 0,
		"gift_token_value_sent": 0.7000000000000001,
		"gift_token_value_received": 0,
		"boost_token_allotment": 25,
		"buff_time_value": 38309.730632150175,
		"teamwork_score": 0.31141672867206993,
		"counted_in_season": false,
		"season_id": "",
		"time_cheats": 0,
		"version": "cxp-v0.2.0",
		"evaluation_start_time": 1696778185.855627,
		"status": 3
	}
}
*/
