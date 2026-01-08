package boost

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"math"
	"os"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"

	"google.golang.org/protobuf/proto"
)

// LoadContractData will load contract data from a file
func LoadContractData(filename string) {

	var EggIncContractsLoaded []ei.EggIncContract
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Handle the error appropriately, e.g., logging or taking corrective actions
			log.Printf("Failed to close: %v", err)
		}
	}()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&EggIncContractsLoaded)
	if err != nil {
		log.Print(err)
		//return
	}

	var EggIncContractsNew []ei.EggIncContract
	//EggIncContractsAllNew := make(map[string]ei.EggIncContract, 800)
	contractProtoBuf := &ei.Contract{}
	for _, c := range EggIncContractsLoaded {
		rawDecodedText, _ := base64.StdEncoding.DecodeString(c.Proto)
		err = proto.Unmarshal(rawDecodedText, contractProtoBuf)
		if err != nil {
			log.Fatalln("Failed to parse contract:", err)
		}
		expirationTime := int64(math.Round(contractProtoBuf.GetExpirationTime()))
		contractTime := time.Unix(expirationTime, 0)

		contract := PopulateContractFromProto(contractProtoBuf)

		if contract.CoopAllowed && contractTime.After(time.Now().UTC()) {
			EggIncContractsNew = append(EggIncContractsNew, contract)
		}

		// Only add completely new contracts to this list
		if existingContract, exists := ei.EggIncContractsAll[c.ID]; !exists || contract.ValidFrom.After(existingContract.ValidFrom) {
			ei.EggIncContractsAll[c.ID] = contract
		}

	}
	ei.EggIncContracts = EggIncContractsNew

	/*
		// Call the function to write the estimated durations to a CSV file
		err = WriteEstimatedDurationsToCSV("estimated_durations.csv")
		if err != nil {
			log.Fatal(err)
		}
	*/
}

const originalContractValidDuration = 21 * 86400
const legacyContractValidDuration = 7 * 86400

// PopulateContractFromProto will populate a contract from a protobuf
func PopulateContractFromProto(contractProtoBuf *ei.Contract) ei.EggIncContract {
	var c ei.EggIncContract
	c.ID = contractProtoBuf.GetIdentifier()

	// Create a protobuf for the contract
	//contractBin, _ := proto.Marshal(contractProtoBuf)
	//c.Proto = base64.StdEncoding.EncodeToString(contractBin)

	expirationTime := int64(math.Round(contractProtoBuf.GetExpirationTime()))
	contractTime := time.Unix(expirationTime, 0)

	c.PeriodicalAPI = false // Default this to false
	c.MaxCoopSize = int(contractProtoBuf.GetMaxCoopSize())
	c.ChickenRunCooldownMinutes = int(contractProtoBuf.GetChickenRunCooldownMinutes())
	c.Name = contractProtoBuf.GetName()
	c.Description = contractProtoBuf.GetDescription()
	c.Egg = int32(contractProtoBuf.GetEgg())

	c.LengthInSeconds = int(contractProtoBuf.GetLengthSeconds())
	c.ModifierIHR = 1.0
	c.ModifierELR = 1.0
	c.ModifierSR = 1.0
	c.ModifierHabCap = 1.0
	c.ContractVersion = 2
	c.Ultra = contractProtoBuf.GetCcOnly()
	c.SeasonID = contractProtoBuf.GetSeasonId()
	c.HasPE = false

	/*
		if c.SeasonID == "fall_2025" || strings.Contains(c.SeasonID, "2026") {
			c.SeasonalScoring = ei.SeasonalScoringNerfed
		}
	*/
	c.SeasonalScoring = ei.SeasonalScoringNerfed

	if contractProtoBuf.GetStartTime() == 0 {

		if contractProtoBuf.Leggacy == nil || contractProtoBuf.GetLeggacy() {
			c.ValidFrom = contractTime.Add(-time.Duration(c.LengthInSeconds-legacyContractValidDuration) * time.Second)
		} else {
			c.ValidFrom = contractTime.Add(-time.Duration(c.LengthInSeconds-originalContractValidDuration) * time.Second)
		}

	} else {
		c.ValidFrom = time.Unix(int64(contractProtoBuf.GetStartTime()), 0)
	}
	c.ValidUntil = contractTime
	c.CoopAllowed = contractProtoBuf.GetCoopAllowed()

	if c.Egg == int32(ei.Egg_CUSTOM_EGG) {
		c.EggName = contractProtoBuf.GetCustomEggId()
	} else {
		c.EggName = ei.Egg_name[c.Egg]
	}

	c.MinutesPerToken = int(contractProtoBuf.GetMinutesPerToken())
	c.Grade = make([]ei.ContractGrade, 6)
	for _, s := range contractProtoBuf.GetGradeSpecs() {
		grade := int(s.GetGrade())
		c.TargetAmount = nil

		//		if grade == ei.Contract_GRADE_AAA {
		for _, g := range s.GetGoals() {
			c.TargetAmount = append(c.TargetAmount, g.GetTargetAmount())
			c.LengthInSeconds = int(s.GetLengthSeconds())
			if g.GetRewardType() == ei.RewardType_EGGS_OF_PROPHECY {
				c.HasPE = true
			}
		}
		c.ModifierIHR = 1.0
		c.ModifierELR = 1.0
		c.ModifierSR = 1.0
		c.ModifierHabCap = 1.0
		c.ModifierEarnings = 1.0
		c.ModifierAwayEarnings = 1.0
		c.ModifierVehicleCost = 1.0
		c.ModifierHabCost = 1.0
		c.ModifierResearchCost = 1.0
		for _, mod := range s.GetModifiers() {
			switch mod.GetDimension() {

			case ei.GameModifier_EARNINGS:
				c.ModifierEarnings = mod.GetValue()
			case ei.GameModifier_AWAY_EARNINGS:
				c.ModifierAwayEarnings = mod.GetValue()
			case ei.GameModifier_INTERNAL_HATCHERY_RATE:
				c.ModifierIHR = mod.GetValue()
			case ei.GameModifier_EGG_LAYING_RATE:
				c.ModifierELR = mod.GetValue()
			case ei.GameModifier_SHIPPING_CAPACITY:
				c.ModifierSR = mod.GetValue()
			case ei.GameModifier_HAB_CAPACITY:
				c.ModifierHabCap = mod.GetValue()
			case ei.GameModifier_VEHICLE_COST:
				c.ModifierVehicleCost = mod.GetValue()
			case ei.GameModifier_HAB_COST:
				c.ModifierHabCost = mod.GetValue()
			case ei.GameModifier_RESEARCH_COST:
				c.ModifierResearchCost = mod.GetValue()
			}
		}
		//		}
		c.Grade[grade].TargetAmount = c.TargetAmount
		c.Grade[grade].ModifierIHR = c.ModifierIHR
		c.Grade[grade].ModifierELR = c.ModifierELR
		c.Grade[grade].ModifierSR = c.ModifierSR
		c.Grade[grade].ModifierHabCap = c.ModifierHabCap
		c.Grade[grade].ModifierEarnings = c.ModifierEarnings
		c.Grade[grade].ModifierAwayEarnings = c.ModifierAwayEarnings
		c.Grade[grade].ModifierVehicleCost = c.ModifierVehicleCost
		c.Grade[grade].ModifierHabCost = c.ModifierHabCost
		c.Grade[grade].ModifierResearchCost = c.ModifierResearchCost
		c.Grade[grade].LengthInSeconds = c.LengthInSeconds

		c.Grade[grade].EstimatedDuration, c.Grade[grade].EstimatedDurationLower, _, _ = getContractDurationEstimate(c, c.TargetAmount[len(c.TargetAmount)-1], float64(c.MaxCoopSize), c.LengthInSeconds,
			c.ModifierSR, c.ModifierELR, c.ModifierHabCap, false)

		gradeKey := ei.Contract_PlayerGrade_name[int32(grade)]
		if gradeMult, ok := ei.GradeMultiplier[gradeKey]; ok {
			c.Grade[grade].BasePoints = 1.0 + (1.0/259200.0*float64(c.LengthInSeconds))*float64(gradeMult)
			goalsCompleted := 1.0
			c.Grade[grade].BasePoints = 187.5 * float64(gradeMult) * goalsCompleted
		}

		BTA := math.Floor(float64(c.Grade[grade].EstimatedDuration.Minutes() / float64(c.MinutesPerToken)))
		c.Grade[grade].TargetTval = 3.0
		if BTA > 42.0 {
			c.Grade[grade].TargetTval = 0.07 * BTA
		}
		BTALower := math.Floor(float64(c.Grade[grade].EstimatedDurationLower.Minutes() / float64(c.MinutesPerToken)))
		c.Grade[grade].TargetTvalLower = 3.0
		if BTALower > 42.0 {
			c.Grade[grade].TargetTvalLower = 0.07 * BTALower
		}
		c.TargetTval = c.Grade[grade].TargetTval
		c.TargetTvalLower = c.Grade[grade].TargetTvalLower
	}
	if c.TargetAmount == nil {
		c.TargetAmount = nil
		for _, g := range contractProtoBuf.GetGoals() {
			c.ContractVersion = 1
			c.TargetAmount = append(c.TargetAmount, g.GetTargetAmount())
		}
		//log.Print("No target amount found for contract ", c.ID)
	}
	if c.LengthInSeconds > 0 {
		d := time.Duration(c.LengthInSeconds) * time.Second
		days := d.Hours() / 24.0 // 2 days
		c.LengthInDays = int(days)
		c.ChickenRuns = int(min(20.0, math.Ceil((days*float64(c.MaxCoopSize))/2.0)))
		if c.SeasonalScoring == ei.SeasonalScoringNerfed {
			c.ChickenRuns = min(20, c.MaxCoopSize-1)
		}

	}
	// Duration estimate
	if len(c.TargetAmount) != 0 {
		/*
			if hasModifier {
				log.Printf("Coop Name: %s, ID: %s, Modifiers: IHR: %f, ELR: %f, SR: %f, HabCap: %f\n",
					c.Name, c.ID, c.ModifierIHR, c.ModifierELR, c.ModifierSR, c.ModifierHabCap)
			}*/
		debug := false

		if config.IsDevBot() && c.ID == "quant-blitz" {
			debug = true
		}

		c.EstimatedDuration, c.EstimatedDurationLower, c.EstimatedDurationMax, c.EstimatedDurationSIAB = getContractDurationEstimate(c, c.TargetAmount[len(c.TargetAmount)-1], float64(c.MaxCoopSize), c.LengthInSeconds,
			c.ModifierSR, c.ModifierELR, c.ModifierHabCap, debug)
	}
	/*


		// Create a Base score with no teamwork multipliers
		scoreBase := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			0, 0, 0)
		diffCR := (float64(scoreBase) * 0.06) / float64(eiContract.ChickenRuns)
		if eiContract.SeasonalScoring == 1 {
			diffCR = (float64(scoreBase) * 0.05) / float64(eiContract.ChickenRuns)
		}

		// Calculate a score with only the Buffs included
		var crBuilder strings.Builder
		minScore := calculateContractScore(eiContract.SeasonalScoring, grade,
			eiContract.MaxCoopSize,
			eiContract.Grade[grade].TargetAmount[len(eiContract.Grade[grade].TargetAmount)-1],
			contribution[i],
			eiContract.Grade[grade].LengthInSeconds,
			contractDurationSeconds,
			B, 0, 0)

		for maxCR := eiContract.ChickenRuns; maxCR >= 0; maxCR-- {
			// Each CR is worth 6% of base score divided by maxCR
			crBuilder.WriteString(fmt.Sprintf("%d:%d ", maxCR, minScore+int64(math.Ceil(float64(maxCR)*diffCR))))
		}
	*/

	if c.ContractVersion == 2 {
		fairShare := 1.00
		if c.SeasonalScoring == ei.SeasonalScoringNerfed {
			fairShare = 1.00
		}
		if c.MaxCoopSize == 1 {
			fairShare = 1.0
			c.CxpMax = float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				c.EstimatedDurationMax, // Use faster duration at a 1.0 modifier
				fairShare,              // Fair Share, first booster
				100, 30,                // SIAB 100%, 30 minutes
				20, 0, // Deflector %, minutes reduction
				0,     // All Chicken Runs - Post CRT
				0, 0)) // Tokens Sent a lot and received a little.

			c.Cxp = float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				c.EstimatedDurationSIAB, // Use faster duration at a 1.0 modifier
				fairShare,               // Fair Share, first booster
				100, 30,                 // SIAB 100%, 30 minutes
				20, 0, // Deflector %, minutes reduction
				0,     // All Chicken Runs - Post CRT
				0, 0)) // Tokens Sent a lot and received a little.

			baseScore := float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				c.EstimatedDurationLower, // Use faster duration at a 1.0 modifier
				fairShare,                // Fair Share, first booster
				0, 0,                     // SIAB 0%, 0 minutes
				0, 0, // Deflector %, minutes reduction
				0,     // All Chicken Runs - used for diff Calc
				0, 0)) // Tokens Sent a lot and received a little.

			c.CxpRunDelta = (float64(baseScore) * 0.06) / float64(c.ChickenRuns)
			if c.SeasonalScoring == ei.SeasonalScoringNerfed {
				c.CxpRunDelta = (float64(baseScore) * 0.05) / float64(c.ChickenRuns)
			}
			c.CxpBuffOnly = float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				c.EstimatedDurationLower, // Use faster duration at a 1.0 modifier
				fairShare,                // Fair Share, first booster
				100, 30,                  // SIAB 100%, 30 minutes
				20, 0, // Deflector %, minutes reduction
				0,     // All Chicken Runs - Post CRT
				0, 0)) // Tokens Sent a lot and received a little.
		} else {
			c.CxpMax = float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				c.EstimatedDurationMax, // Use faster duration at a 1.0 modifier
				fairShare,              // Fair Share, first booster
				100, 30,                // SIAB 100%, 30 minutes
				20, 0, // Deflector %, minutes reduction
				c.ChickenRuns, // All Chicken Runs - Post CRT
				100, 5))       // Tokens Sent a lot and received a little.

			c.CxpMaxSiab = float64(getContractScoreEstimateWithDuration(c, ei.Contract_GRADE_AAA,
				c.EstimatedDurationSIAB,                     // Use faster duration at a 1.0 modifier
				fairShare,                                   // Fair Share, first booster
				100, int(c.EstimatedDurationSIAB.Minutes()), // SIAB 100%, full duration
				20, 0, // Deflector %, minutes reduction
				c.ChickenRuns, // All Chicken Runs - Post CRT
				100, 5))       // Tokens Sent a lot and received a little.

			c.Cxp = float64(getContractScoreEstimate(c, ei.Contract_GRADE_AAA,
				true, 1.0, // Use faster duration at a 1.0 modifier
				fairShare, // Fair Share, first booster
				100, 30,   // SIAB 100%, 30 minutes
				20, 0, // Deflector %, minutes reduction
				c.MaxCoopSize-1, // All Chicken Runs - Post CRT
				100, 5))         // Tokens Sent a lot and received a little.

			baseScore := float64(getContractScoreEstimate(c, ei.Contract_GRADE_AAA,
				true, 1.0, // Use faster duration at a 1.0 modifier
				fairShare, // Fair Share, first booster
				0, 0,      // SIAB 0%, 0 minutes
				0, 0, // Deflector %, minutes reduction
				0,     // All Chicken Runs - used for diff Calc
				0, 0)) // Tokens Sent a lot and received a little.

			c.CxpRunDelta = (float64(baseScore) * 0.06) / float64(c.ChickenRuns)
			if c.SeasonalScoring == ei.SeasonalScoringNerfed {
				c.CxpRunDelta = (float64(baseScore) * 0.05) / float64(c.ChickenRuns)
			}
			c.CxpBuffOnly = float64(getContractScoreEstimate(c, ei.Contract_GRADE_AAA,
				true, 1.0, // Use faster duration at a 1.0 modifier
				fairShare, // Fair Share, first booster
				100, 20,   // SIAB 100%, 20 minutes
				20, 0, // Deflector %, minutes reduction
				0,       // All Chicken Runs - Post CRT
				100, 5)) // Tokens Sent a lot and received a little.
		}
	}

	return c
}

func updateContractWithEggIncData(contract *Contract) {
	for _, cc := range ei.EggIncContracts {
		if cc.ID == contract.ContractID {
			contract.CoopSize = cc.MaxCoopSize
			contract.LengthInSeconds = cc.LengthInSeconds
			contract.ChickenRuns = cc.ChickenRuns
			contract.EstimatedDuration = cc.EstimatedDuration
			contract.Name = cc.Name
			contract.Description = cc.Description
			contract.EggName = cc.EggName
			contract.ChickenRunCooldownMinutes = cc.ChickenRunCooldownMinutes
			contract.MinutesPerToken = cc.MinutesPerToken
			contract.Ultra = cc.Ultra
			contract.SeasonalScoring = cc.SeasonalScoring
			break
		}
	}
}
