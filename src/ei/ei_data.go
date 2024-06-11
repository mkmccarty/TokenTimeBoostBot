package ei

import "time"

// EggIncContract is a raw contract data for Egg Inc
type EggIncContract struct {
	ID                        string `json:"id"`
	Proto                     string `json:"proto"`
	Name                      string
	Description               string
	Egg                       int32
	EggName                   string
	MaxCoopSize               int
	TargetAmount              []float64
	TargetAmountq             []float64
	ChickenRuns               int
	LengthInSeconds           int
	ChickenRunCooldownMinutes int
	MinutesPerToken           int
	ContractDurationInDays    int
	EstimatedDuration         time.Duration
}

// EggIncContracts holds a list of all contracts, newest is last
var EggIncContracts []EggIncContract

// EggIncContractsAll holds a list of all contracts, newest is last
var EggIncContractsAll map[string]EggIncContract

func init() {
	EggIncContractsAll = make(map[string]EggIncContract)
}
