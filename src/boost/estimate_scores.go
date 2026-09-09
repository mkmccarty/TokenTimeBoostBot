package boost

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// ContractScore holds all the relevant information for calculating and displaying contract scores for contracts
type ContractScore struct {
	coopID                   string
	contractID               string
	cxpversion               int
	grade                    int
	coopSize                 int
	crRequirement            int
	contractLengthSeconds    int
	targetGoal               float64
	activeContractDurSeconds float64
	playerParamters          []PlayerScoreParameters
}

// PlayerScoreParameters holds player specific parameters for calculating scores
type PlayerScoreParameters struct {
	name         string
	contribution float64
	buff         float64
}

// csEstimateParams holds the parameters parsed from the /cs-estimate command
type csEstimateParams struct {
	contractID string
	coopID     string
	srMode     bool
	imageTable bool
	ephemeral  bool
}

// GetSlashCsEstimates returns the slash command for estimating scores
func GetSlashCsEstimates(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Provide a Contract Score estimates for a running contract")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "contract-id",
			Description:  "Select a contract-id",
			Autocomplete: true,
		},
		dc.StringOption{
			Name:        "coop-id",
			Description: "Your coop-id",
		},
		dc.BoolOption{
			Name:        "sr-mode",
			Description: "Display detailed score breakdown for Speedrun Predictions, default is false",
		},
		dc.BoolOption{
			Name:        "as-image",
			Description: "Render table as an image instead of text. Default is false (sticky).",
		},
		dc.BoolOption{
			Name:        "private-reply",
			Description: "Response visibility, default is public",
		},
	}
	return &command
}

func parseCsEstimateParams(e *dc.CommandEvent) csEstimateParams {
	var (
		contractID string
		coopID     string
	)

	if opt, ok := e.OptString("contract-id"); ok {
		contractID = strings.ToLower(strings.ReplaceAll(opt, " ", ""))
	}
	if opt, ok := e.OptString("coop-id"); ok {
		coopID = strings.ToLower(strings.ReplaceAll(opt, " ", ""))
	}
	ephemeral := false
	if opt, ok := e.OptBool("private-reply"); ok && opt {
		ephemeral = true
	}
	srMode := false
	if opt, ok := e.OptBool("sr-mode"); ok {
		srMode = opt
	}
	userID := e.UserID()
	imageTable := farmerstate.GetMiscSettingFlag(userID, "as-image")
	if opt, ok := e.OptBool("as-image"); ok {
		imageTable = opt
		farmerstate.SetMiscSettingFlag(userID, "as-image", imageTable)
	}

	return csEstimateParams{
		contractID: contractID,
		coopID:     coopID,
		srMode:     srMode,
		imageTable: imageTable,
		ephemeral:  ephemeral,
	}
}

// HandleCsEstimatesCommand handles the estimate scores command.
func HandleCsEstimatesCommand(e *dc.CommandEvent) {
	p := parseCsEstimateParams(e)

	_ = e.Defer(p.ephemeral)

	runCsEstimate(e, p)
}

// HandleCsEstimateButtons handles button interactions for /cs-estimate.
func HandleCsEstimateButtons(e *dc.ComponentEvent) {
	const ttl = 5 * time.Minute
	expired := false
	if messageID := e.MessageID(); messageID != "" {
		createdAt, err := dc.SnowflakeTimestamp(messageID)
		if err != nil {
			log.Println("Error parsing message timestamp:", err)
			return
		}
		expired = time.Since(createdAt) > ttl
	}

	// Keep everything but the trailing action row, which is what this handler
	// takes away once it has run.
	kept := e.MessageComponentsWithoutActionRows()

	reaction := strings.Split(e.CustomID(), "#")
	if err := e.DeferUpdate(); err != nil {
		log.Println(err)
	}

	if !expired {
		// Handle the button actions
		action := reaction[1]
		contractHash := reaction[2]
		contract := FindContractByHash(contractHash)
		if contract == nil {
			log.Println("Contract not found for hash:", contractHash)
			return
		}

		// Is the user in the contract?
		userID := e.UserID()
		if !UserInContract(contract, userID) {
			// Ignore if the user isn't in the contract
			return
		}

		switch action {
		case "completionping":
			go sendCompletionPing(e, contract, userID)
		case "checkinping":
			//go SendCheckinPings(s, i, contract)
		default:
			// default to close
		}
	}

	// Remove the buttons regardless of expiration
	if err := e.EditResponse(dc.Message{Components: kept}); err != nil {
		log.Println(err)
	}
}

func runCsEstimate(e *dc.CommandEvent, p csEstimateParams) {
	contractID := p.contractID
	coopID := p.coopID

	// Look for the contract in the channel to determine buttons
	foundContractHash := ""
	contract := FindContract(e.ChannelID())
	if contract != nil {
		foundContractHash = contract.ContractHash
	}

	if contractID == "" || coopID == "" {
		if contract == nil {
			commandLink := bottools.GetFormattedCommand("cs-estimate")
			if commandLink == "" {
				commandLink = "/cs-estimate"
			}
			_ = e.Followup(dc.Message{
				Ephemeral: true,
				Components: []dc.LayoutComponent{
					dc.TextDisplay{Content: fmt.Sprintf("No contract found in this channel. Run %s in a channel with an active contract, or provide a `contract-id` and `coop-id`.", commandLink)},
				},
			})
			return
		}
		contractID = contract.ContractID
		coopID = strings.ToLower(contract.CoopID)
	}

	// Get coopStatus with the given contractID and coopID
	userID := e.UserID()
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	str, fields, contractScore := DownloadCoopStatusTeamwork(e.ChannelID(), contractID, coopID, true, eiID)
	if fields == nil || strings.HasSuffix(str, "no such file or directory") || strings.HasPrefix(str, "No grade found") {
		if sendErr := e.Followup(dc.Message{
			Ephemeral: p.ephemeral,
			Components: []dc.LayoutComponent{
				dc.TextDisplay{Content: str},
			},
		}); sendErr != nil {
			log.Println("Cs-estsimate: FollowupMessageCreate:", sendErr)
		}
		return
	}

	// Build projected rows
	durationDays := max(1, int(math.Ceil(float64(contractScore.contractLengthSeconds)/86400.0)))

	// Teamwork from CRs and depricated Tval
	CR := calculateChickenRunTeamwork(
		contractScore.cxpversion,
		contractScore.coopSize,
		durationDays,
		contractScore.crRequirement,
	)
	T := 0.0

	fairContribution := 0.0
	if contractScore.coopSize > 0 {
		fairContribution = contractScore.targetGoal / float64(contractScore.coopSize)
	}

	rows := make([]srRow, 0, len(contractScore.playerParamters))

	for _, player := range contractScore.playerParamters {
		contrRatio := 0.0
		if fairContribution > 0 {
			contrRatio = player.contribution / fairContribution
		}

		cxp, teamworkScore := calculateSRScores(
			contractScore.cxpversion,
			contractScore.grade,
			contractScore.coopSize,
			contractScore.contractLengthSeconds,
			contractScore.targetGoal,
			contractScore.activeContractDurSeconds,
			CR,
			T,
			player.contribution,
			player.buff,
		)

		btv := int(player.buff * contractScore.activeContractDurSeconds)

		rows = append(rows, srRow{
			name:       player.name,
			cxp:        cxp,
			contrRatio: contrRatio,
			teamwork:   teamworkScore,
			btv:        btv,
			diffStr:    "-",
			diffVal:    0,
			diffOK:     false,
		})
	}

	// Sort rows by CXP desc
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].cxp > rows[j].cxp
	})

	// Phase 1: Render table without making archive fetches, send initial response with just estimates
	title := "## Projected Scores"
	if p.srMode {
		title = "## Projected SR Scores"
	}

	var components []dc.LayoutComponent
	var files []dc.File

	if p.imageTable {
		imgBytes, err := RenderScoreTableImage(rows, p.srMode, false)
		if err != nil {
			log.Printf("Error rendering score table image: %v", err)
		} else if len(imgBytes) > 0 {
			files = append(files, dc.File{
				Name:        "cs_estimate.png",
				ContentType: "image/png",
				Reader:      bytes.NewReader(imgBytes),
			})
			mediaItem := dc.MediaItem{URL: "attachment://cs_estimate.png"}
			components = []dc.LayoutComponent{
				dc.TextDisplay{Content: str},
				dc.TextDisplay{Content: title},
				dc.MediaGallery{Items: []dc.MediaItem{mediaItem}},
			}
		}
	}

	if len(components) == 0 {
		tableFast := RenderScoreTableANSI(rows, p.srMode, false)
		components = []dc.LayoutComponent{
			dc.TextDisplay{Content: str},
			dc.TextDisplay{Content: title},
			dc.TextDisplay{Content: tableFast},
		}
	}

	msg, sendErr := e.FollowupMessage(dc.Message{
		Ephemeral:  p.ephemeral,
		Components: components,
		Files:      files,
	})
	if sendErr != nil || msg == nil {
		if sendErr != nil {
			log.Println("FollowupMessageCreate:", sendErr)
		}
		return
	}

	// Phase 2: fetch archives, compute diffs, then edit to add Diff + buttons
	actionRow, haveActionRow := buildCsEstimateActionRow(foundContractHash)

	rowsCopy := make([]srRow, len(rows))
	copy(rowsCopy, rows)

	names := make([]string, len(rowsCopy))
	for i, row := range rowsCopy {
		names[i] = row.name
	}

	msgID := msg.ID
	srmode := p.srMode
	imageTable := p.imageTable

	go func() {
		archives, fetched, missing, err := GetContractArchivesForNames(
			names,
			"cxp_v0_2_0",
			false,
			true,
		)
		if err != nil {
			log.Println("GetContractArchivesForNames:", err)
		}
		if len(missing) > 0 {
			log.Println("Missing EI links for:", strings.Join(missing, ", "))
		}

		prev, foundPrev := PrevContractScoresFromArchives(archives, fetched, contractScore.contractID, contractScore.coopID)

		var anyDiffs bool
		for idx := range rowsCopy {
			if idx < len(foundPrev) && foundPrev[idx] {
				dv := int64(math.Round(float64(rowsCopy[idx].cxp) - prev[idx]))
				rowsCopy[idx].diffVal = dv
				rowsCopy[idx].diffStr = fmt.Sprintf("%+d", dv)
				rowsCopy[idx].diffOK = true
				anyDiffs = true
			} else {
				rowsCopy[idx].diffVal = 0
				rowsCopy[idx].diffStr = "-"
				rowsCopy[idx].diffOK = false
			}
		}

		var edited []dc.LayoutComponent
		var editFiles []dc.File

		if imageTable {
			imgBytes, err := RenderScoreTableImage(rowsCopy, srmode, anyDiffs)
			if err != nil {
				log.Printf("Error rendering score table image edit: %v", err)
			} else if len(imgBytes) > 0 {
				editFiles = append(editFiles, dc.File{
					Name:        "cs_estimate.png",
					ContentType: "image/png",
					Reader:      bytes.NewReader(imgBytes),
				})
				mediaItem := dc.MediaItem{URL: "attachment://cs_estimate.png"}
				edited = []dc.LayoutComponent{
					dc.TextDisplay{Content: str},
					dc.TextDisplay{Content: title},
					dc.MediaGallery{Items: []dc.MediaItem{mediaItem}},
				}
			}
		}

		if len(edited) == 0 {
			tableFinal := RenderScoreTableANSI(rowsCopy, srmode, anyDiffs)
			edited = []dc.LayoutComponent{
				dc.TextDisplay{Content: str},
				dc.TextDisplay{Content: title},
				dc.TextDisplay{Content: tableFinal},
			}
		}

		if haveActionRow {
			edited = append(edited, actionRow)
		}

		if editErr := e.EditFollowup(msgID, dc.Message{
			Components: edited,
			Files:      editFiles,
		}); editErr != nil {
			log.Println("FollowupMessageEdit:", editErr)
		}
	}()
}

func buildCsEstimateActionRow(foundContractHash string) (dc.LayoutComponent, bool) {
	if foundContractHash == "" {
		return nil, false
	}

	buttonConfigs := []struct {
		label  string
		style  dc.ButtonStyle
		action string
	}{
		{"Completion Ping", dc.ButtonPrimary, "completionping"},
		// {"Check-in Pings", dc.ButtonSecondary, "checkinping"},
		{"Close", dc.ButtonDanger, "close"},
	}

	buttons := make([]dc.InteractiveComponent, 0, len(buttonConfigs))
	for _, cfg := range buttonConfigs {
		buttons = append(buttons, dc.Button{
			Label:    cfg.label,
			Style:    cfg.style,
			CustomID: fmt.Sprintf("csestimate#%s#%s", cfg.action, foundContractHash),
		})
	}

	return dc.ActionRow{Components: buttons}, true
}

// sendCompletionPing sends a ping with the estimated completion time of the contract
func sendCompletionPing(e *dc.ComponentEvent, contract *Contract, userID string) {

	allFinalized := true
	if time.Now().After(contract.EstimatedEndTime) {
		// If the contract has ended, we check if all users have checked in (finalized)
		eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
		if coopStatus, _, _, err := ei.GetCoopStatus(contract.ContractID, contract.CoopID, eiID); err == nil && coopStatus != nil {
			for _, contributor := range coopStatus.GetContributors() {
				if !contributor.GetFinalized() {
					allFinalized = false
					break
				}
			}
		}
	}

	//Check if the contract is still ongoing (or not all users have checked in)
	if time.Now().After(contract.EstimatedEndTime) && allFinalized {
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   "The contract has already ended. Ping not sent.",
		})
		return
	}

	// Get the role mention from the active contract
	roleMention := contract.Location[0].RoleMention
	if roleMention == "" {
		roleMention = "@here"
	}

	requesterMention := userID
	if b := contract.Boosters[userID]; b != nil {
		requesterMention = b.Mention
	}

	var message string
	if time.Now().After(contract.EstimatedEndTime) {
		message = fmt.Sprintf("%s, The contract **%s** `%s` has ended! Please check-in to finalize scores.\n-# Ping requested by %s",
			roleMention,
			contract.Name,
			contract.CoopID,
			requesterMention,
		)
	} else {
		message = fmt.Sprintf("%s, The contract **%s** `%s` will complete on\n## %s at %s!\nPlease set an alarm or leave your devices on!\n-# Ping requested by %s",
			roleMention,
			contract.Name,
			contract.CoopID,
			bottools.WrapTimestamp(contract.EstimatedEndTime.Unix(), bottools.TimestampLongDate),
			bottools.WrapTimestamp(contract.EstimatedEndTime.Unix(), bottools.TimestampLongTime),
			requesterMention,
		)
	}

	err := e.Followup(dc.Message{
		Content: message,
		AllowedMentions: &dc.AllowedMentions{
			Parse: []dc.MentionType{dc.MentionRoles, dc.MentionUsers},
		},
	})

	if err != nil {
		log.Println("Error sending completion ping:", err)
		_ = e.Followup(dc.Message{
			Ephemeral: true,
			Content:   "Error sending completion ping.",
		})
		return
	}
}

// calculateSRScores calculates contract score and returns player values
func calculateSRScores(cxpversion, grade, coopSize, contractLength int, targetGoal, contractDurationSeconds, CR, T float64, contribution, B float64) (int64, float64) {
	basePoints := 1.0
	durationPoints := 1.0 / 259200.0
	score := basePoints + durationPoints*float64(contractLength)

	gradeMultiplier := ei.GradeMultiplier[ei.Contract_PlayerGrade_name[int32(grade)]]
	score *= gradeMultiplier

	completionFactor := 1.0
	score *= completionFactor

	ratio := contribution / (targetGoal / float64(coopSize))
	contributionFactor := 0.0
	if ratio <= 2.5 {
		contributionFactor = 1 + 3*math.Pow(ratio, 0.15)
	} else {
		contributionFactor = 0.02221*min(ratio, 12.5) + 4.386486
	}
	score *= contributionFactor

	completionTimeBonus := 1.0 + 4.0*math.Pow((1.0-float64(contractDurationSeconds)/float64(contractLength)), 3)
	score *= completionTimeBonus

	teamworkScore := getPredictedTeamwork(cxpversion, B, CR, T)
	teamworkBonus := 1.0 + 0.19*teamworkScore

	score *= teamworkBonus
	score *= float64(187.5)
	if cxpversion == ei.SeasonalScoringNerfed {
		score *= 1.05 // 5% bonus for whatever
	}

	return int64(math.Ceil(score)), teamworkScore
}

// GetContractArchivesForNames fetches contract archives for a list of player names concurrently, returning index-aligned slices for (archives, fetched, missing).
func GetContractArchivesForNames(names []string, cxpVersion string, forceRefresh bool, okayToSave bool,
) (archives [][]*ei.LocalContract, fetched []bool, missing []string, err error) {

	// Get encryption key for decrypting EI IDs
	encKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode enc key: %w", err)
	}

	n := len(names)
	archives = make([][]*ei.LocalContract, n)
	fetched = make([]bool, n)
	if n == 0 {
		return archives, fetched, nil, nil
	}

	maxParallelJobs := max(min(n, maxParallel), 1)

	// Ensure cache directory exists for writing individual archives if needed
	cacheDir := "ttbb-data/eiuserdata"
	if config.IsDevBot() && okayToSave {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			log.Printf("GetContractArchivesForNames: ensure cache dir failed: %v", err)
		}
	}

	type job struct {
		i    int
		name string
	}

	jobsCh := make(chan job, n)
	var (
		wg       sync.WaitGroup
		muMiss   sync.Mutex
		missingL = make([]string, 0, n)
	)

	worker := func() {
		defer wg.Done()
		for j := range jobsCh {
			name := j.name
			if name == "" {
				continue
			}

			discordID, e := farmerstate.GetDiscordUserIDFromEiIgnExact(name)
			if e != nil && config.IsDevBot() {
				log.Printf("GetContractArchivesForNames: IGN->discordID lookup failed name=%q: %v", name, e)
			}
			if discordID == "" {
				muMiss.Lock()
				missingL = append(missingL, name)
				muMiss.Unlock()
				continue
			}

			eiCipherB64 := farmerstate.GetMiscSettingString(discordID, "encrypted_ei_id")
			if eiCipherB64 == "" {
				muMiss.Lock()
				missingL = append(missingL, name)
				muMiss.Unlock()
				continue
			}

			cipherBytes, e := base64.StdEncoding.DecodeString(eiCipherB64)
			if e != nil {
				muMiss.Lock()
				missingL = append(missingL, name)
				muMiss.Unlock()
				continue
			}

			plain, e := config.DecryptCombined(encKey, cipherBytes)
			if e != nil {
				muMiss.Lock()
				missingL = append(missingL, name)
				muMiss.Unlock()
				continue
			}

			eiID := string(plain)
			if len(eiID) != 18 || !strings.HasPrefix(eiID, "EI") {
				muMiss.Lock()
				missingL = append(missingL, name)
				muMiss.Unlock()
				continue
			}

			archive, _ := ei.GetContractArchiveFromAPI(eiID, discordID, forceRefresh, okayToSave)

			archives[j.i] = archive
			fetched[j.i] = true

		}
	}

	wg.Add(maxParallelJobs)
	for range maxParallelJobs {
		go worker()
	}

	for i, name := range names {
		jobsCh <- job{i: i, name: name}
	}
	close(jobsCh)
	wg.Wait()

	return archives, fetched, missingL, nil
}

// prevCXPFromArchive returns the previous CXP for this contract, returning (0, false) if not found or any issue with the archive
func prevCXPFromArchive(arch []*ei.LocalContract, contractID, currentCoopID string) (float64, bool) {
	for _, lc := range arch {
		ev := lc.GetEvaluation()
		if ev == nil {
			continue
		}
		lcContractID := lc.GetContractIdentifier()
		if lcContractID == "" && lc.GetContract() != nil {
			lcContractID = lc.GetContract().GetIdentifier()
		} else if lcContractID == "" {
			lcContractID = ev.GetContractIdentifier()
		}
		if lcContractID != contractID {
			continue
		}
		if ev.GetCoopIdentifier() == currentCoopID {
			continue
		}
		return ev.GetCxp(), true
	}
	return 0, false
}

// PrevContractScoresFromArchives returns previous contract scores for players whose archives were fetched.
func PrevContractScoresFromArchives(archives [][]*ei.LocalContract, fetched []bool, contractID, currentCoopID string) (prev []float64, foundPrev []bool) {

	n := len(archives)
	prev = make([]float64, n)
	foundPrev = make([]bool, n)

	for i := range n {
		if fetched != nil && i < len(fetched) && !fetched[i] {
			continue
		}
		if old, ok := prevCXPFromArchive(archives[i], contractID, currentCoopID); ok {
			prev[i] = old
			foundPrev[i] = true
		}
	}
	return prev, foundPrev
}

// --------------------
// ANSI Score Table
// --------------------

const (
	srnameW  = 12
	srCxpW   = 6
	srContrW = 5          // 1.000
	srTeamW  = 4          // .664
	srBtvW   = 7          // max 7 figures
	srDiffW  = srCxpW + 1 // include sign
)

// Row data the table renders
type srRow struct {
	name       string
	cxp        int64
	contrRatio float64
	teamwork   float64
	btv        int
	diffStr    string
	diffVal    int64
	diffOK     bool
}

// Used only for "highlight max" coloring
type srPeaks struct {
	cxp        int64
	contrRatio float64
	teamwork   float64
	btv        int
}

// RenderScoreTableANSI renders the score table in an ANSI code block, highlighting the max value in each column. If includeDiff is true, includes the Diff column.
func RenderScoreTableANSI(rows []srRow, srmode bool, includeDiff bool) string {
	peaks := computePeaks(rows)

	// header
	header := bottools.AlignString("Name", srnameW, bottools.StringAlignLeft) +
		"|" + bottools.AlignString("CXP", srCxpW, bottools.StringAlignCenterRight)

	if srmode {
		header += "|" + bottools.AlignString("Contr", srContrW, bottools.StringAlignCenterRight) +
			"|" + bottools.AlignString("TmWk", srTeamW, bottools.StringAlignCenterRight) +
			"|" + bottools.AlignString("BTV", srBtvW, bottools.StringAlignCenterRight)
	}

	if includeDiff {
		header += "|" + bottools.AlignString("Diff", srDiffW, bottools.StringAlignCenterRight)
	}

	var b strings.Builder
	b.WriteString("```ansi\n")
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("—", bottools.VisibleLenANSI(header)))
	b.WriteByte('\n')

	for _, r := range rows {
		// Name
		line := bottools.FitString(ei.NormalizePlayerNameForDisplay(r.name), srnameW, bottools.StringAlignLeft)

		// CXP (blue if max)
		cxpColor := ""
		if r.cxp == peaks.cxp {
			cxpColor = "blue"
		}
		cxpTxt := bottools.FitString(fmt.Sprintf("%d", r.cxp), srCxpW, bottools.StringAlignRight)
		line += "|" + bottools.CellANSI(cxpTxt, cxpColor, srCxpW, true)

		if srmode {
			// Contr (blue if max)
			contrColor := ""
			if approxEqual(r.contrRatio, peaks.contrRatio) {
				contrColor = "blue"
			}
			contrTxt := bottools.FitString(fmt.Sprintf("%.3f", r.contrRatio), srContrW, bottools.StringAlignRight)
			line += "|" + bottools.CellANSI(contrTxt, contrColor, srContrW, true)

			// TmWk (blue if max)
			tmwkColor := ""
			if approxEqual(r.teamwork, peaks.teamwork) {
				tmwkColor = "blue"
			}
			tmwkTxt := bottools.FitString(formatTmWk(r.teamwork), srTeamW, bottools.StringAlignRight)
			line += "|" + bottools.CellANSI(tmwkTxt, tmwkColor, srTeamW, true)

			// BTV (blue if max)
			btvColor := ""
			if r.btv == peaks.btv {
				btvColor = "blue"
			}
			btvTxt := bottools.FitString(fmt.Sprintf("%d", r.btv), srBtvW, bottools.StringAlignRight)
			line += "|" + bottools.CellANSI(btvTxt, btvColor, srBtvW, true)
		}

		if includeDiff {
			diffColor := ""
			if r.diffOK {
				if r.diffVal > 0 {
					diffColor = "green"
				} else if r.diffVal < 0 {
					diffColor = "red"
				}
			}
			diffTxt := bottools.FitString(r.diffStr, srDiffW, bottools.StringAlignRight)
			line += "|" + bottools.CellANSI(diffTxt, diffColor, srDiffW, true)
		}

		b.WriteString(line)
		b.WriteByte('\n')
	}

	b.WriteString("```")
	return b.String()
}

func computePeaks(rows []srRow) srPeaks {
	p := srPeaks{}
	for _, r := range rows {
		if r.cxp > p.cxp {
			p.cxp = r.cxp
		}
		if r.contrRatio > p.contrRatio {
			p.contrRatio = r.contrRatio
		}
		if r.teamwork > p.teamwork {
			p.teamwork = r.teamwork
		}
		if r.btv > p.btv {
			p.btv = r.btv
		}
	}
	return p
}

func formatTmWk(f float64) string {
	if math.Abs(f) < 1 {
		s := fmt.Sprintf("%.3f", f)
		if f >= 0 && strings.HasPrefix(s, "0") {
			return s[1:] // 0.664 -> .664
		}
		if f < 0 && strings.HasPrefix(s, "-0") {
			return "-" + s[2:] // -0.664 -> -.664
		}
		return s
	}
	return fmt.Sprintf("%.2f", f)
}

// RenderScoreTableImage renders the score table as a PNG image using RenderTableImage.
func RenderScoreTableImage(rows []srRow, srmode bool, includeDiff bool) ([]byte, error) {
	peaks := computePeaks(rows)
	cols := []TableImageColumn{
		{Label: "Name", Align: bottools.StringAlignLeft},
		{Label: "CXP", Align: bottools.StringAlignRight},
	}
	if srmode {
		cols = append(cols,
			TableImageColumn{Label: "Contr", Align: bottools.StringAlignRight},
			TableImageColumn{Label: "TW", Align: bottools.StringAlignRight},
			TableImageColumn{Label: "BTV", Align: bottools.StringAlignRight},
		)
	}
	if includeDiff {
		cols = append(cols, TableImageColumn{Label: "Diff", Align: bottools.StringAlignRight})
	}

	var tableRows []TableImageRow
	for _, r := range rows {
		cxpColor := ""
		if r.cxp == peaks.cxp {
			cxpColor = "blue"
		}
		cells := []TableImageCell{
			{Text: ei.NormalizePlayerNameForDisplay(r.name), Color: ""},
			{Text: fmt.Sprintf("%d", r.cxp), Color: cxpColor},
		}
		if srmode {
			contrColor := ""
			if approxEqual(r.contrRatio, peaks.contrRatio) {
				contrColor = "blue"
			}
			tmwkColor := ""
			if approxEqual(r.teamwork, peaks.teamwork) {
				tmwkColor = "blue"
			}
			btvColor := ""
			if r.btv == peaks.btv {
				btvColor = "blue"
			}
			cells = append(cells,
				TableImageCell{Text: fmt.Sprintf("%.3f", r.contrRatio), Color: contrColor},
				TableImageCell{Text: formatTmWk(r.teamwork), Color: tmwkColor},
				TableImageCell{Text: fmt.Sprintf("%d", r.btv), Color: btvColor},
			)
		}
		if includeDiff {
			diffColor := ""
			if r.diffOK {
				if r.diffVal > 0 {
					diffColor = "green"
				} else if r.diffVal < 0 {
					diffColor = "red"
				}
			}
			cells = append(cells, TableImageCell{Text: r.diffStr, Color: diffColor})
		}
		tableRows = append(tableRows, TableImageRow{Cells: cells})
	}

	return RenderTableImage(cols, tableRows)
}
