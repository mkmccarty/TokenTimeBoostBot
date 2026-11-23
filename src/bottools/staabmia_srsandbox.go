package bottools

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// SRSandboxModifierType identifies the active contract modifier type in SR Sandbox.
// Order matches the mod-name select in https://srsandbox-staabmia.netlify.app/scripts.js.
type SRSandboxModifierType int

const (
	SRSandboxModHabSpace SRSandboxModifierType = 0
	SRSandboxModIHR      SRSandboxModifierType = 1
	SRSandboxModShipRate SRSandboxModifierType = 2
	SRSandboxModLayRate  SRSandboxModifierType = 3
)

type srSandboxArtifact struct {
	name          string
	slots         int
	elrMult       float64
	srMult        float64
	ihrMult       float64
	deflectorPerc float64
	siabPerc      float64
	chickMult     float64
	isEmpty       bool
}

// Item lists matching https://srsandbox-staabmia.netlify.app/scripts.js exactly.
var (
	srItemsMetro = []srSandboxArtifact{
		{name: "T4L Metro", slots: 3, elrMult: 1.35, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T4E Metro", slots: 2, elrMult: 1.3, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T4R Metro", slots: 1, elrMult: 1.27, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T4C Metro", slots: 0, elrMult: 1.25, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T3E Metro", slots: 2, elrMult: 1.2, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "3 Slot", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T3R Metro", slots: 1, elrMult: 1.17, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T3C Metro", slots: 0, elrMult: 1.15, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T2R Metro", slots: 1, elrMult: 1.12, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T2C Metro", slots: 0, elrMult: 1.1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T1C Metro", slots: 0, elrMult: 1.05, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T4L SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 100, chickMult: 1},
		{name: "T4E SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 90, chickMult: 1},
		{name: "T4R SIAB", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 80, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}

	srItemsComp = []srSandboxArtifact{
		{name: "T4L Compass", slots: 2, elrMult: 1, srMult: 1.5, ihrMult: 1, chickMult: 1},
		{name: "T4E Compass", slots: 2, elrMult: 1, srMult: 1.4, ihrMult: 1, chickMult: 1},
		{name: "T4R Compass", slots: 1, elrMult: 1, srMult: 1.35, ihrMult: 1, chickMult: 1},
		{name: "T4C Compass", slots: 0, elrMult: 1, srMult: 1.3, ihrMult: 1, chickMult: 1},
		{name: "T3R Compass", slots: 1, elrMult: 1, srMult: 1.22, ihrMult: 1, chickMult: 1},
		{name: "T3C Compass", slots: 0, elrMult: 1, srMult: 1.2, ihrMult: 1, chickMult: 1},
		{name: "T2C Compass", slots: 0, elrMult: 1, srMult: 1.1, ihrMult: 1, chickMult: 1},
		{name: "T1C Compass", slots: 0, elrMult: 1, srMult: 1.05, ihrMult: 1, chickMult: 1},
		{name: "3 Slot", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T4L SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 100, chickMult: 1},
		{name: "T4E SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 90, chickMult: 1},
		{name: "T4R SIAB", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 80, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}

	srItemsGusset = []srSandboxArtifact{
		{name: "T4L Gusset", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.25},
		{name: "T4E Gusset", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.22},
		{name: "T2E Gusset", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.12},
		{name: "3 Slot", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "T4C Gusset", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.2},
		{name: "T3R Gusset", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.16},
		{name: "T3C Gusset", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.15},
		{name: "T2C Gusset", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.1},
		{name: "T1C Gusset", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1.05},
		{name: "T4L SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 100, chickMult: 1},
		{name: "T4E SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 90, chickMult: 1},
		{name: "T4R SIAB", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 80, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}

	srItemsDefl = []srSandboxArtifact{
		{name: "T4L Defl.", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 20, chickMult: 1},
		{name: "T4E Defl.", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 19, chickMult: 1},
		{name: "T4R Defl.", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 17, chickMult: 1},
		{name: "T4C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 15, chickMult: 1},
		{name: "T3R Defl.", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 13, chickMult: 1},
		{name: "T3C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 12, chickMult: 1},
		{name: "T2C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 8, chickMult: 1},
		{name: "T1C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 5, chickMult: 1},
		{name: "3 Slot", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}

	srItemsChal = []srSandboxArtifact{
		{name: "T4L Chalice", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1.4, chickMult: 1},
		{name: "T4E Chalice", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1.35, chickMult: 1},
		{name: "T4C Chalice", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.3, chickMult: 1},
		{name: "T3E Chalice", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1.25, chickMult: 1},
		{name: "T3R Chalice", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1.23, chickMult: 1},
		{name: "T3C Chalice", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.2, chickMult: 1},
		{name: "T2E Chalice", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1.15, chickMult: 1},
		{name: "T2C Chalice", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.1, chickMult: 1},
		{name: "T1C Chalice", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.05, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}

	srItemsMonocle = []srSandboxArtifact{
		{name: "T4L Monocle", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1.3, chickMult: 1},
		{name: "T4E Monocle", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1.25, chickMult: 1},
		{name: "T4C Monocle", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.2, chickMult: 1},
		{name: "T3C Monocle", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.15, chickMult: 1},
		{name: "T2C Monocle", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.1, chickMult: 1},
		{name: "T1C Monocle", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1.05, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}

	srItemsIHRDefl = []srSandboxArtifact{
		{name: "T4L Defl.", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 20, chickMult: 1},
		{name: "T4E Defl.", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 19, chickMult: 1},
		{name: "T4R Defl.", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 17, chickMult: 1},
		{name: "T4C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 15, chickMult: 1},
		{name: "T3R Defl.", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 13, chickMult: 1},
		{name: "T3C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 12, chickMult: 1},
		{name: "T2C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 8, chickMult: 1},
		{name: "T1C Defl.", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, deflectorPerc: 5, chickMult: 1},
		{name: "3 Slot", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "2 Slot", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}

	srItemsIHRSIAB = []srSandboxArtifact{
		{name: "T4L SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 100, chickMult: 1},
		{name: "T4E SIAB", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 90, chickMult: 1},
		{name: "T4R SIAB", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 80, chickMult: 1},
		{name: "T4C SIAB", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 70, chickMult: 1},
		{name: "T3R SIAB", slots: 1, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 60, chickMult: 1},
		{name: "T3C SIAB", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, siabPerc: 50, chickMult: 1},
		{name: "3 Slot", slots: 3, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "2 Slot", slots: 2, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1},
		{name: "Empty", slots: 0, elrMult: 1, srMult: 1, ihrMult: 1, chickMult: 1, isEmpty: true},
	}
)

const (
	srSPMaxArtifacts       = 2
	srSPAssumedDeflPercent = 20.0
	srSPAssumedSIABPercent = 100.0
	srBaseChickens         = 11340000000.0
	srBaseShipRate         = 2978359222414.5 * 2400.0
	srBaseELRPerChicken    = 332640.0
	srBaseIHR              = 7440.0
)

func srItemAt(list []srSandboxArtifact, idx int) srSandboxArtifact {
	if idx < 0 || idx >= len(list) {
		return list[len(list)-1]
	}
	return list[idx]
}

// SRSandboxPlayerInput defines a single player's inputs for SR Sandbox simulation.
type SRSandboxPlayerInput struct {
	Name         string
	Tokens       int
	TE           int
	Mirror       bool
	Colleggtible bool
	Sink         bool
	Creator      bool

	// 0-based indices into the SR Sandbox item lists (matching GetSandboxItemIndices / GetSandboxIHRItemIndices).
	MetroIdx        int
	CompassIdx      int
	GussetIdx       int
	DeflectorIdx    int
	ChaliceIdx      int
	MonocleIdx      int
	IHRDeflectorIdx int
	IHRSIABIdx      int
}

// NewSRSandboxPlayerInputFromCodes builds an SRSandboxPlayerInput from string-encoded fields
// (such as those stored in boost.SandboxPlayer).
func NewSRSandboxPlayerInputFromCodes(
	name, tokensStr, teStr string,
	mirror, colleggtible, sink, creator bool,
	metro, comp, gusset, defl, chalice, monocle, ihrDefl, ihrSIAB string,
) SRSandboxPlayerInput {
	parseCode := func(s string, def int) int {
		if s == "" {
			return def
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return def
		}
		return v
	}
	return SRSandboxPlayerInput{
		Name:            name,
		Tokens:          parseCode(tokensStr, 5),
		TE:              parseCode(teStr, 50),
		Mirror:          mirror,
		Colleggtible:    colleggtible,
		Sink:            sink,
		Creator:         creator,
		MetroIdx:        parseCode(metro, 0),
		CompassIdx:      parseCode(comp, 0),
		GussetIdx:       parseCode(gusset, 0),
		DeflectorIdx:    parseCode(defl, 0),
		ChaliceIdx:      parseCode(chalice, 0),
		MonocleIdx:      parseCode(monocle, 0),
		IHRDeflectorIdx: parseCode(ihrDefl, 0),
		IHRSIABIdx:      parseCode(ihrSIAB, 0),
	}
}

// SRSandboxConfig holds all simulation parameters matching https://srsandbox-staabmia.netlify.app/scripts.js.
type SRSandboxConfig struct {
	ContractName      string
	CRTRun            bool // Chicken Runs Maxed?
	TokenToggle       bool // Token Value Maxed?
	GG                bool // Generous Gifts?
	EarlySwap         bool // Swap once shipping capped?
	CxpToggle         bool // New Runs (after 12/22/2025, new2p0)
	JoinDelaySeconds  float64
	MinPerTokenGift   float64 // minutes/token gift/player (default 13)
	DurationSeconds   float64
	TargetEggAmount   float64 // total target eggs (e.g. 500e15)
	TokenTimerMinutes float64
	ModifierType      SRSandboxModifierType
	ModifierMult      float64
	BTVTarget         float64 // default 1.875 when CxpToggle, 2.0 otherwise
	Players           []SRSandboxPlayerInput
}

// SRSandboxPlayerResult holds a single player's calculated simulation results.
type SRSandboxPlayerResult struct {
	Name               string
	TE                 int
	LayRate            float64
	ShipRate           float64
	DeliveryRate       float64
	NumTach            int
	NumQuant           int
	Chickens           float64
	MaxChickens        float64
	HasBeforeSwap      bool
	BeforeLayRate      float64
	BeforeShipRate     float64
	BeforeDeliveryRate float64
	BeforeNumTach      int
	BeforeNumQuant     int
	BeforeChickens     float64
	ContributionRatio  float64
	TimeToBoost        float64
	BoostingTime       float64
	BTVRatio           float64
	Teamwork           float64
	CS                 float64
	DeflectorPercent   float64
	IsSink             bool
	IsSPSwap           bool
	SIABActive         bool
	Swapped            bool
	SwapTime           float64
	SinkCR             int
	SinkTokens         int
}

// SRSandboxResults holds the full simulation output matching the Results section of SR Sandbox.
type SRSandboxResults struct {
	ContractName              string
	Success                   bool
	CoopRateBeforeSwap        float64
	CoopRate                  float64
	SIABActive                bool
	SIABSwapTime              float64
	CompletionTime            float64
	Duration                  float64
	MaxCS                     float64
	MeanCS                    float64
	MinCS                     float64
	TotalDeflector            float64
	UnusedDeflector           float64
	UnusedDeflectorBeforeSwap float64
	New2p0                    bool
	Players                   []SRSandboxPlayerResult
}

type srRates struct {
	layRate      float64
	shipRate     float64
	deliveryRate float64
	numTach      int
	numQuant     int
}

type srBeforeSwap struct {
	chickens         float64
	maxChickens      float64
	numTach          int
	numQuant         int
	deflectorPercent float64
	layRate          float64
	shipRate         float64
	deliveryRate     float64
	ratesAtMaxHab    srRates
}

type srSimPlayer struct {
	name   string
	tokens int
	te     int

	// stats
	chickens          float64
	maxChickens       float64
	eggsDelivered     float64
	deflectorPercent  float64
	siabPercent       float64
	otherDefl         float64
	timeToBoost       float64
	boostingTime      float64
	numTach           int
	numQuant          int
	boostMulti        float64
	btv               float64
	contributionRatio float64
	btvRat            float64
	tw                float64
	cs                float64
	spSwapBTV         float64
	spSwapWindow      float64
	spCreditDefl      float64
	spCreditSiab      float64
	chickensNeeded    float64
	swapTime          float64

	// rates
	ihr             float64
	deliveryRate    float64
	baseShip        float64
	baseELR         float64
	layRate         float64
	shipRate        float64
	ihrLaySet       float64
	maxDeliveryRate float64

	// flags
	needsMirror bool
	isSink      bool
	isCreator   bool
	maxHab      bool
	siabActive  bool
	collegg     bool
	spSwap      bool
	swapped     bool

	// Lay-rate artifacts in new site order: [0]=Defl, [1]=Metro, [2]=Compass, [3]=Gusset
	artifacts  [4]srSandboxArtifact
	beforeSwap *srBeforeSwap
}

func (p *srSimPlayer) updateChickens() {
	increase := p.ihr * 12.0 * p.boostMulti / 60.0
	p.chickens = math.Min(p.chickens+increase, p.maxChickens)
	if p.chickens == p.maxChickens {
		p.maxHab = true
	}
}

func (p *srSimPlayer) updateDeliveryRate() {
	p.layRate = p.chickens * srBaseELRPerChicken * (1.0 + p.otherDefl/100.0)
	p.shipRate = p.baseShip
	p.deliveryRate = math.Min(p.layRate, p.baseShip)
}

func (p *srSimPlayer) updateEggsDelivered(updateRate float64) {
	p.eggsDelivered += updateRate * p.deliveryRate / 3600.0
}

func (p *srSimPlayer) updateEggsDeliveredSIAB(updateRate float64) {
	if p.beforeSwap != nil {
		p.eggsDelivered += updateRate * p.beforeSwap.deliveryRate / 3600.0
	}
}

func (p *srSimPlayer) updateBTV(updateRate float64, new2p0 bool) {
	var btvRate float64
	if new2p0 {
		btvRate = 12.5*math.Min(p.deflectorPercent, 12.0) + 0.75*math.Min(p.siabPercent, 50.0)
	} else {
		btvRate = 7.5 * (p.deflectorPercent + p.siabPercent/10.0)
	}
	p.btv += updateRate * btvRate / 100.0
}

func srOptimizeStones(elr, sr float64, totSlots int) (float64, float64, int, int) {
	numTach := 0
	numQuant := 0
	for i := 0; i < totSlots; i++ {
		if elr < sr {
			elr *= 1.05
			numTach++
		} else {
			sr *= 1.05
			numQuant++
		}
	}
	return elr, sr, numTach, numQuant
}

func (p *srSimPlayer) calcLaySetRates(chickens float64) srRates {
	elr := chickens * p.baseELR
	sr := p.baseShip
	slots := 0
	for i := 0; i < 4; i++ {
		a := p.artifacts[i]
		elr *= a.elrMult
		sr *= a.srMult
		slots += a.slots
	}
	elr *= (1.0 + p.otherDefl/100.0)
	layRate, shipRate, nTach, nQuant := srOptimizeStones(elr, sr, slots)
	return srRates{
		layRate:      layRate,
		shipRate:     shipRate,
		deliveryRate: math.Min(layRate, shipRate),
		numTach:      nTach,
		numQuant:     nQuant,
	}
}

func (p *srSimPlayer) applyRates(r srRates) {
	p.layRate = r.layRate
	p.shipRate = r.shipRate
	p.deliveryRate = r.deliveryRate
	p.numTach = r.numTach
	p.numQuant = r.numQuant
}

func (p *srSimPlayer) updateSwapTarget() {
	const eps = 1e-9
	maxChickens := p.maxChickens
	maxDelivery := p.calcLaySetRates(maxChickens).deliveryRate

	elrPerChicken := p.baseELR
	srBase := p.baseShip
	slots := 0
	for i := 0; i < 4; i++ {
		a := p.artifacts[i]
		elrPerChicken *= a.elrMult
		srBase *= a.srMult
		slots += a.slots
	}
	elrPerChicken *= (1.0 + p.otherDefl/100.0)

	tach := 0
	for t := slots; t >= 0; t-- {
		if srBase*math.Pow(1.05, float64(slots-t)) >= maxDelivery*(1.0-eps) {
			tach = t
			break
		}
	}

	needed := math.Ceil(maxDelivery / (elrPerChicken * math.Pow(1.05, float64(tach))))
	if math.IsInf(needed, 0) || math.IsNaN(needed) || needed >= maxChickens*(1.0-eps) {
		needed = maxChickens
	}
	p.chickensNeeded = math.Max(0, needed)
	p.maxDeliveryRate = maxDelivery
}

func (p *srSimPlayer) isSettled() bool {
	if p.maxHab {
		return true
	}
	return p.swapped && p.deliveryRate >= p.maxDeliveryRate*(1.0-1e-9)
}

func (p *srSimPlayer) capAtSwapTarget(prevChickens float64) {
	target := p.chickensNeeded
	if prevChickens < target && p.chickens > target {
		p.chickens = target
		p.maxHab = target >= p.maxChickens
	}
}

func (p *srSimPlayer) updateArtis() {
	defl := 0.0
	siab := 0.0
	for i := 0; i < 4; i++ {
		defl += p.artifacts[i].deflectorPerc
		siab += p.artifacts[i].siabPerc
	}
	p.deflectorPercent = defl
	p.siabPercent = siab
}

func (p *srSimPlayer) swapToLaySet(tElapsed, duration float64) {
	p.swapped = true
	p.swapTime = tElapsed
	if p.timeToBoost > 0 && p.timeToBoost < duration {
		p.boostingTime = tElapsed - p.timeToBoost
	} else {
		p.boostingTime = tElapsed
	}
	p.ihr = p.ihrLaySet
	p.updateArtis()
}

func (p *srSimPlayer) rateAtMaxHab() srRates {
	if p.swapped && p.chickens < p.maxChickens {
		return p.calcLaySetRates(p.maxChickens)
	}
	return srRates{
		layRate:      p.layRate,
		shipRate:     p.shipRate,
		deliveryRate: p.deliveryRate,
		numTach:      p.numTach,
		numQuant:     p.numQuant,
	}
}

func srCalcBoostMulti(tokens int) float64 {
	switch tokens {
	case 1:
		return (4 * 10) * 2
	case 2:
		return 100 + 4*10
	case 3:
		return (100 + 3*10) * 2
	case 4:
		return 1000 + 4*10
	case 5:
		return (1000 + 3*10) * 2
	case 6:
		return (1000 + 2*10) * (2 + 2)
	case 7:
		return (1000 + 10) * (2 + 2 + 2)
	case 8:
		return (1000 + 3*10) * 10
	case 9:
		return (1000 + 2*10) * (10 + 2)
	case 10:
		return (1000 + 10) * (10 + 2 + 2)
	case 11:
		return 1000 * (10 + 2 + 2 + 2)
	case 12:
		return (1000 + 3*10) * 50
	default:
		return 50
	}
}

func srCalcSinkCR(numPlayers int, durDays float64) int {
	if numPlayers < 3 {
		return numPlayers - 1
	}
	targetCR := int(math.Ceil(durDays * float64(numPlayers) / 2.0))
	if targetCR > 20 {
		targetCR = 20
	}
	runSessions := int(math.Ceil(float64(targetCR-1) / float64(numPlayers-2)))
	return runSessions * (numPlayers - 1)
}

func srGetTeamwork(btvRat float64, numPlayers int, durDays float64, crt int, T float64, new2p0 bool) float64 {
	B := math.Min(btvRat, 2.0)
	crtF := math.Min(float64(crt), 20.0)
	fCR := math.Max(12.0/float64(numPlayers)/durDays, 0.3)
	CR := math.Min(fCR*crtF, 6.0)
	if new2p0 {
		if numPlayers > 1 {
			CR = 5.0
		} else {
			CR = 0.0
		}
		T = 0.0
	}
	return (5.0*B + CR + T) / 19.0
}

func srGetCS(contributionRatio, originalLength, completionTime, tw float64, new2p0 bool) float64 {
	cs := 1.0 + originalLength/259200.0
	cs *= 7.0
	var fac float64
	if contributionRatio > 2.5 {
		fac = 0.02221*math.Min(contributionRatio, 12.5) + 4.386486
	} else {
		fac = 3.0*math.Pow(contributionRatio, 0.15) + 1.0
	}
	cs *= fac
	cs *= 4.0*math.Pow(1.0-completionTime/originalLength, 3.0) + 1.0
	cs *= (0.19*tw + 1.0)
	if new2p0 {
		cs *= 1.05
	}
	return math.Ceil(cs * 187.5)
}

type srSimState struct {
	updateRate     float64
	eggsDelivered  float64
	tElapsed       float64
	tokensUsed     int
	numberBoosting int
	allMaxHabs     bool
	allBoosting    bool
	siabSwapTime   float64
}

type srSIABStatus struct {
	creatorSIABActive bool
	siabActive        bool
	deflRateMax       float64
	siabRateMax       float64
}

func srBuildSimPlayers(cfg SRSandboxConfig) []*srSimPlayer {
	players := make([]*srSimPlayer, 0, len(cfg.Players))
	totDeflector := 0.0

	srMod := 1.0
	if cfg.ModifierType == SRSandboxModShipRate && cfg.ModifierMult > 0 {
		srMod = cfg.ModifierMult
	}
	elrMod := 1.0
	if cfg.ModifierType == SRSandboxModLayRate && cfg.ModifierMult > 0 {
		elrMod = cfg.ModifierMult
	}
	habMod := 1.0
	if cfg.ModifierType == SRSandboxModHabSpace && cfg.ModifierMult > 0 {
		habMod = cfg.ModifierMult
	}
	ihrMod := 1.0
	if cfg.ModifierType == SRSandboxModIHR && cfg.ModifierMult > 0 {
		ihrMod = cfg.ModifierMult
	}

	for i, in := range cfg.Players {
		deflArt := srItemAt(srItemsDefl, in.DeflectorIdx)
		metroArt := srItemAt(srItemsMetro, in.MetroIdx)
		compArt := srItemAt(srItemsComp, in.CompassIdx)
		gussetArt := srItemAt(srItemsGusset, in.GussetIdx)
		layArts := [4]srSandboxArtifact{deflArt, metroArt, compArt, gussetArt}

		ihrDeflArt := srItemAt(srItemsIHRDefl, in.IHRDeflectorIdx)
		ihrSIABArt := srItemAt(srItemsIHRSIAB, in.IHRSIABIdx)
		monocleArt := srItemAt(srItemsMonocle, in.MonocleIdx)
		chalArt := srItemAt(srItemsChal, in.ChaliceIdx)
		ihrArts := [4]srSandboxArtifact{ihrDeflArt, ihrSIABArt, monocleArt, chalArt}

		// Initial deflector & SIAB percents come from IHR set
		deflPerc := 0.0
		siabPerc := 0.0
		boostedUsed := 0
		ihrUsed := 0
		for j := 0; j < 4; j++ {
			if !layArts[j].isEmpty {
				boostedUsed++
			}
			if !ihrArts[j].isEmpty {
				ihrUsed++
			}
			deflPerc += ihrArts[j].deflectorPerc
			siabPerc += ihrArts[j].siabPerc
		}
		totDeflector += deflPerc

		// Standard permit credit check (getStandardPermitCredit)
		spSwap := false
		spCreditDefl := 0.0
		spCreditSiab := 0.0
		if boostedUsed > 0 && ihrUsed > 0 && boostedUsed <= srSPMaxArtifacts && ihrUsed <= srSPMaxArtifacts {
			if deflPerc == 0 {
				spCreditDefl = srSPAssumedDeflPercent
			}
			if siabPerc == 0 {
				spCreditSiab = srSPAssumedSIABPercent
			}
			if spCreditDefl > 0 || spCreditSiab > 0 {
				spSwap = true
			}
		}

		// Max chickens (getMaxChickens)
		chick := srBaseChickens
		for j := 0; j < 4; j++ {
			chick *= layArts[j].chickMult
		}
		chick *= habMod
		if in.Colleggtible {
			chick *= 1.05
		}
		maxChickens := math.Floor(chick)

		// IHR (calcIHR)
		ihr := srBaseIHR
		for j := 0; j < 4; j++ {
			ihr *= ihrArts[j].ihrMult * math.Pow(1.04, float64(ihrArts[j].slots))
		}
		ihr *= ihrMod
		if in.Colleggtible {
			ihr *= 1.05
		}
		teMult := math.Pow(1.01, float64(in.TE))
		ihr = math.Floor(ihr * teMult)

		// Bare lay-set IHR (calcIHRLaySet)
		ihrLay := srBaseIHR * ihrMod
		if in.Colleggtible {
			ihrLay *= 1.05
		}
		ihrLay = math.Floor(ihrLay * teMult)

		colleggShip := 1.0
		colleggELR := 1.0
		if in.Colleggtible {
			colleggShip = 1.1025
			colleggELR = 1.1025
		}

		name := in.Name
		if name == "" {
			name = fmt.Sprintf("Player %d", i)
		}

		players = append(players, &srSimPlayer{
			name:             name,
			tokens:           in.Tokens,
			te:               in.TE,
			boostMulti:       srCalcBoostMulti(0),
			maxChickens:      maxChickens,
			deflectorPercent: deflPerc,
			siabPercent:      siabPerc,
			spCreditDefl:     spCreditDefl,
			spCreditSiab:     spCreditSiab,
			timeToBoost:      cfg.DurationSeconds,
			ihr:              ihr,
			ihrLaySet:        ihrLay,
			baseShip:         srBaseShipRate * colleggShip * srMod,
			baseELR:          srBaseELRPerChicken * elrMod * colleggELR,
			needsMirror:      in.Mirror,
			isSink:           in.Sink,
			isCreator:        in.Creator,
			collegg:          in.Colleggtible,
			spSwap:           spSwap,
			artifacts:        layArts,
		})
	}

	for _, p := range players {
		p.otherDefl = totDeflector - p.deflectorPercent
	}

	return players
}

func srCalcRate(playerIndex int, chickens float64, players []*srSimPlayer) {
	p := players[playerIndex]
	elr := chickens * p.baseELR
	sr := p.baseShip
	totDeflector := 0.0
	totSlotsAvailable := 0
	useLaySet := p.swapped || p.maxChickens == chickens
	if useLaySet {
		for i := 0; i < 4; i++ {
			elr *= p.artifacts[i].elrMult
			sr *= p.artifacts[i].srMult
			totSlotsAvailable += p.artifacts[i].slots
		}
	}
	for i := range players {
		if i != playerIndex {
			totDeflector += players[i].deflectorPercent
		}
	}
	elr *= (1.0 + totDeflector/100.0)
	numTach := p.numTach
	numQuant := p.numQuant
	if useLaySet {
		elr, sr, numTach, numQuant = srOptimizeStones(elr, sr, totSlotsAvailable)
	}
	p.layRate = elr
	p.shipRate = sr
	p.deliveryRate = math.Min(elr, sr)
	p.chickens = chickens
	p.numTach = numTach
	p.numQuant = numQuant
}

func srSimUpdateRate(players []*srSimPlayer, cfg SRSandboxConfig, state srSimState, totalDeflector *float64, simTime float64, crtFlag bool) srSimState {
	timeToTokenGift := cfg.MinPerTokenGift * 60.0
	if timeToTokenGift <= 0 {
		timeToTokenGift = 13.0 * 60.0
	}
	tokenTimer := math.Max(cfg.TokenTimerMinutes*60.0, 1.0)
	gg := 1.0
	if cfg.GG {
		gg = 2.0
	}
	numPlayers := len(players)

	state.eggsDelivered = 0
	for _, p := range players {
		state.eggsDelivered += p.eggsDelivered
	}

	for state.eggsDelivered < cfg.TargetEggAmount && state.tElapsed < simTime && !state.allMaxHabs {
		totNotMaxHabs := 0
		*totalDeflector = 0
		updateOtherDefl := false

		for _, player := range players {
			if !player.isSettled() && (!crtFlag || player.isCreator) {
				prevChickens := player.chickens
				player.updateChickens()
				if cfg.EarlySwap {
					player.capAtSwapTarget(prevChickens)
				}
				totNotMaxHabs++

				if player.swapped {
					player.applyRates(player.calcLaySetRates(player.chickens))
				} else {
					player.updateDeliveryRate()
					hitSwapTarget := cfg.EarlySwap && player.chickens >= player.chickensNeeded
					if player.maxHab || hitSwapTarget {
						player.swapToLaySet(state.tElapsed, cfg.DurationSeconds)
						updateOtherDefl = true
					}
				}
			}
			*totalDeflector += player.deflectorPercent
			player.updateEggsDelivered(state.updateRate)
			player.updateBTV(state.updateRate, cfg.CxpToggle)
		}

		if updateOtherDefl {
			for _, player := range players {
				player.otherDefl = *totalDeflector - player.deflectorPercent
			}
			for index, player := range players {
				if !crtFlag || player.isCreator {
					srCalcRate(index, player.chickens, players)
					player.deliveryRate = math.Min(player.layRate, player.shipRate)
				}
			}
			if cfg.EarlySwap {
				for _, player := range players {
					player.updateSwapTarget()
				}
			}
		}

		state.allMaxHabs = totNotMaxHabs == 0

		totTokens := int(math.Floor(state.tElapsed*float64(numPlayers)/timeToTokenGift) * gg)
		totTokens += int(math.Floor(state.tElapsed/tokenTimer)) + int(math.Floor((state.tElapsed-cfg.JoinDelaySeconds)/tokenTimer))*(numPlayers-1)

		if !state.allBoosting {
			nextPlayer := players[state.numberBoosting]
			if !crtFlag || nextPlayer.isCreator {
				if nextPlayer.isSettled() {
					nextPlayer.timeToBoost = cfg.DurationSeconds
					state.numberBoosting++
				} else if nextPlayer.tokens <= (totTokens - state.tokensUsed) {
					offset := 0
					if nextPlayer.needsMirror {
						offset = 1
					}
					nextPlayer.boostMulti = srCalcBoostMulti(nextPlayer.tokens - offset)
					state.tokensUsed += nextPlayer.tokens
					nextPlayer.timeToBoost = state.tElapsed
					state.numberBoosting++
				}
			}
			state.allBoosting = state.numberBoosting == len(players)
		}

		state.eggsDelivered = 0
		for _, player := range players {
			if !crtFlag || player.isCreator {
				state.eggsDelivered += player.eggsDelivered
			}
		}

		state.tElapsed += state.updateRate
	}

	return state
}

func srCheckPlayerSIAB(players []*srSimPlayer, new2p0 bool) srSIABStatus {
	btvRateMax := 0.0
	status := srSIABStatus{}
	for _, p := range players {
		if p.siabPercent > 0 {
			status.siabActive = true
			p.siabActive = true
			var deflRate, siabRate float64
			if new2p0 {
				deflRate = 12.5 * math.Min(p.deflectorPercent, 12.0) / 100.0
				siabRate = 0.75 * math.Min(p.siabPercent, 50.0) / 100.0
			} else {
				deflRate = 7.5 * p.deflectorPercent / 100.0
				siabRate = 0.75 * p.siabPercent / 100.0
			}
			if btvRateMax <= deflRate+siabRate {
				btvRateMax = deflRate + siabRate
				status.deflRateMax = deflRate
				status.siabRateMax = siabRate
			}
		}
	}
	return status
}

func srSaveDataBeforeSwap(players []*srSimPlayer) {
	for _, p := range players {
		p.beforeSwap = &srBeforeSwap{
			chickens:         p.chickens,
			maxChickens:      p.maxChickens,
			numTach:          p.numTach,
			numQuant:         p.numQuant,
			deflectorPercent: p.deflectorPercent,
			layRate:          p.layRate,
			shipRate:         p.shipRate,
			deliveryRate:     p.deliveryRate,
			ratesAtMaxHab:    p.rateAtMaxHab(),
		}
	}
}

func srCalcRateSIABRemoved(players []*srSimPlayer) {
	totDeflector := 0.0
	for _, p := range players {
		totDeflector += p.deflectorPercent
	}

	t4lReplacements := [4]srSandboxArtifact{
		srItemsDefl[0],
		srItemsMetro[0],
		srItemsComp[0],
		srItemsGusset[0],
	}

	for _, p := range players {
		elr := p.chickens * p.baseELR
		sr := p.baseShip
		totSlotsAvailable := 0
		atMaxHab := p.maxChickens == p.chickens
		topUp := !atMaxHab && p.swapped
		if atMaxHab || p.swapped {
			for i := 0; i < 4; i++ {
				if p.artifacts[i].siabPerc > 0 {
					p.artifacts[i] = t4lReplacements[i]
					if i == 3 {
						if atMaxHab {
							p.chickens *= p.artifacts[i].chickMult
							elr *= p.artifacts[i].chickMult
						}
						p.maxChickens *= p.artifacts[i].chickMult
					}
				}
				elr *= p.artifacts[i].elrMult
				sr *= p.artifacts[i].srMult
				totSlotsAvailable += p.artifacts[i].slots
			}
		}

		elr *= (1.0 + (totDeflector-p.deflectorPercent)/100.0)
		var numTach, numQuant int
		elr, sr, numTach, numQuant = srOptimizeStones(elr, sr, totSlotsAvailable)

		if topUp {
			p.otherDefl = totDeflector - p.deflectorPercent
			p.updateSwapTarget()
			p.chickens = math.Max(p.chickens, p.chickensNeeded)
			r := p.calcLaySetRates(p.chickens)
			elr, sr, numTach, numQuant = r.layRate, r.shipRate, r.numTach, r.numQuant
		}

		p.layRate = elr
		p.shipRate = sr
		p.deliveryRate = math.Min(elr, sr)
		p.numTach = numTach
		p.numQuant = numQuant
	}
}

func srHandleSIABActive(players []*srSimPlayer, cfg SRSandboxConfig, state *srSimState, rate float64, siabStatus srSIABStatus) float64 {
	rn := 0.0
	srSaveDataBeforeSwap(players)
	srCalcRateSIABRemoved(players)
	for _, p := range players {
		rn += p.deliveryRate / 3600.0
	}

	crtFactor := 0.0
	if siabStatus.creatorSIABActive {
		crtFactor = 1.0
	}

	t0 := state.tElapsed - cfg.JoinDelaySeconds*crtFactor
	r1 := rate / 3600.0
	btvTarget := cfg.BTVTarget
	deflRateMax := siabStatus.deflRateMax
	siabRateMax := siabStatus.siabRateMax

	num := rn*(btvTarget*cfg.JoinDelaySeconds*crtFactor-t0*(siabRateMax+deflRateMax-btvTarget)) - (deflRateMax-btvTarget)*(cfg.TargetEggAmount-state.eggsDelivered)
	den := rn*(siabRateMax+deflRateMax-btvTarget) - (deflRateMax-btvTarget)*r1
	t1 := num / den
	t1 = math.Max(t1, 0)

	tCompleteAtCurrentRate := (cfg.TargetEggAmount - state.eggsDelivered) / (rate / 3600.0)
	rateChange := t1 < tCompleteAtCurrentRate
	t1 = math.Min(t1, tCompleteAtCurrentRate)

	for _, p := range players {
		p.updateEggsDeliveredSIAB(t1)
		p.updateBTV(t1, cfg.CxpToggle)
	}
	eggsDelivered := 0.0
	for _, p := range players {
		eggsDelivered += p.eggsDelivered
	}
	for _, p := range players {
		if p.siabPercent > 0 {
			p.siabPercent = 0
		}
	}

	if rateChange {
		rate = rn * 3600.0
	}

	state.tElapsed += t1
	state.eggsDelivered = eggsDelivered
	state.siabSwapTime = cfg.JoinDelaySeconds + t0 + t1

	return rate
}

func srApplyStandardPermitCredit(players []*srSimPlayer, cfg SRSandboxConfig, completionTime float64) {
	endTime := cfg.DurationSeconds
	if completionTime > 0 {
		endTime = math.Min(completionTime, cfg.DurationSeconds)
	}
	for _, p := range players {
		p.spSwapBTV = 0
		p.spSwapWindow = 0
		if !p.spSwap {
			continue
		}
		window := math.Min(p.timeToBoost, endTime)
		if !(window > 0) {
			continue
		}
		savedSiab := p.siabPercent
		savedDefl := p.deflectorPercent
		savedBtv := p.btv

		p.deflectorPercent = p.spCreditDefl
		p.siabPercent = p.spCreditSiab
		p.btv = 0
		p.updateBTV(window, cfg.CxpToggle)
		credit := p.btv

		p.siabPercent = savedSiab
		p.deflectorPercent = savedDefl
		p.btv = savedBtv + credit
		p.spSwapBTV = credit
		p.spSwapWindow = window
	}
}

func srGetDeflectorDropPerc(players []*srSimPlayer, totDeflector float64, siabCoop bool) (beforeDrop, afterDrop float64) {
	if len(players) < 2 {
		return totDeflector, totDeflector
	}

	val1 := 0.0
	elrDivSrMin := math.Inf(1)
	def := (totDeflector-players[0].deflectorPercent)/100.0 + 1.0
	for _, p := range players {
		r := p.rateAtMaxHab()
		if r.shipRate <= 0 {
			continue
		}
		ratio := r.layRate / r.shipRate
		if ratio < elrDivSrMin {
			elrDivSrMin = ratio
			def = (totDeflector-p.deflectorPercent)/100.0 + 1.0
		}
	}
	if elrDivSrMin >= 1.0 && !math.IsInf(elrDivSrMin, 1) {
		val1 = (def-1.0)*100.0 - (def/elrDivSrMin-1.0)*100.0
	}

	val0 := 0.0
	elrDivSrMin2 := math.Inf(1)
	if siabCoop {
		def2 := (totDeflector-players[0].beforeSwap.deflectorPercent)/100.0 + 1.0
		for _, p := range players {
			if p.beforeSwap == nil {
				continue
			}
			rb := p.beforeSwap.ratesAtMaxHab
			if rb.shipRate <= 0 {
				continue
			}
			ratio := rb.layRate / rb.shipRate
			if ratio < elrDivSrMin2 {
				elrDivSrMin2 = ratio
				def2 = (totDeflector-p.beforeSwap.deflectorPercent)/100.0 + 1.0
			}
		}
		if elrDivSrMin2 >= 1.0 && !math.IsInf(elrDivSrMin2, 1) {
			val0 = (def2-1.0)*100.0 - (def2/elrDivSrMin2-1.0)*100.0
		}
	}

	return math.Min(math.Floor(val0), math.Round(totDeflector)), math.Min(math.Floor(val1), math.Round(totDeflector))
}

// RunSRSandboxSimulation runs the exact contract simulation from https://srsandbox-staabmia.netlify.app/scripts.js.
func RunSRSandboxSimulation(cfg SRSandboxConfig) SRSandboxResults {
	if len(cfg.Players) == 0 {
		return SRSandboxResults{ContractName: cfg.ContractName, Duration: cfg.DurationSeconds, New2p0: cfg.CxpToggle}
	}
	if cfg.MinPerTokenGift <= 0 {
		cfg.MinPerTokenGift = 13
	}
	if cfg.TokenTimerMinutes <= 0 {
		cfg.TokenTimerMinutes = 60
	}
	if cfg.ModifierMult <= 0 {
		cfg.ModifierMult = 1.0
	}
	if cfg.BTVTarget <= 0 {
		if cfg.CxpToggle {
			cfg.BTVTarget = 1.875
		} else {
			cfg.BTVTarget = 2.0
		}
	}

	players := srBuildSimPlayers(cfg)
	state := srSimState{
		updateRate: 1.0,
	}
	results := SRSandboxResults{
		ContractName: cfg.ContractName,
		Duration:     cfg.DurationSeconds,
		New2p0:       cfg.CxpToggle,
	}

	if cfg.EarlySwap {
		for _, p := range players {
			p.updateSwapTarget()
		}
	}

	if players[0].isCreator {
		if state.tElapsed < cfg.JoinDelaySeconds {
			state = srSimUpdateRate(players, cfg, state, &results.TotalDeflector, cfg.JoinDelaySeconds, true)
		}
	} else {
		state.tElapsed = cfg.JoinDelaySeconds
	}

	state = srSimUpdateRate(players, cfg, state, &results.TotalDeflector, cfg.DurationSeconds, false)

	rate := 0.0
	for _, p := range players {
		rate += p.deliveryRate
	}
	if rate == 0 {
		rate = 1.0
	}
	results.CoopRate = rate

	if state.tElapsed <= cfg.DurationSeconds {
		if state.eggsDelivered < cfg.TargetEggAmount {
			siabStatus := srCheckPlayerSIAB(players, cfg.CxpToggle)
			results.SIABActive = siabStatus.siabActive

			rate = 0
			for _, p := range players {
				rate += p.deliveryRate
			}
			if rate == 0 {
				rate = 1.0
			}
			results.CoopRate = rate
			rateOld := rate

			if siabStatus.siabActive {
				results.CoopRateBeforeSwap = rateOld
				rate = srHandleSIABActive(players, cfg, &state, rate, siabStatus)
			}
			results.CoopRateBeforeSwap = rateOld
			results.CoopRate = rate

			tRemaining := (cfg.TargetEggAmount - state.eggsDelivered) / (rate / 3600.0)
			results.CompletionTime = state.tElapsed + tRemaining

			for _, p := range players {
				p.updateEggsDelivered(tRemaining)
				p.updateBTV(tRemaining, cfg.CxpToggle)
			}

			if results.CompletionTime < cfg.DurationSeconds {
				results.Success = true
			}
		}
	}

	results.SIABSwapTime = state.siabSwapTime

	if state.tElapsed <= cfg.DurationSeconds && state.eggsDelivered >= cfg.TargetEggAmount {
		results.CompletionTime = state.tElapsed
		results.Success = true
	}

	srApplyStandardPermitCredit(players, cfg, results.CompletionTime)

	// Calculate CS, teamwork, contribution ratio, BTV ratio, max/mean/min CS (getCSMaxMean)
	numPlayers := len(players)
	fairShare := cfg.TargetEggAmount / float64(numPlayers)
	durDays := cfg.DurationSeconds / 86400.0
	maxCS := 0.0
	meanCS := 0.0
	defMeanCS := 0.0
	minCS := 1e15
	numNoDefl := 0

	for _, p := range players {
		contrib := 0.0
		if fairShare > 0 {
			contrib = p.eggsDelivered / fairShare
		}
		btvRat := 0.0
		if results.CompletionTime > 0 {
			btvRat = p.btv / results.CompletionTime
		}
		T := 0.0
		if cfg.TokenToggle {
			T = 10.0
		}
		if numPlayers == 1 {
			T = 0.0
		}
		crt := numPlayers - 1
		if p.isSink {
			crt = srCalcSinkCR(numPlayers, durDays)
		} else if cfg.CRTRun {
			crt = 20
		}
		if numPlayers == 1 {
			crt = 0
		}
		tw := srGetTeamwork(btvRat, numPlayers, durDays, crt, T, cfg.CxpToggle)
		cs := srGetCS(contrib, cfg.DurationSeconds, results.CompletionTime, tw, cfg.CxpToggle)
		p.contributionRatio = contrib
		p.btvRat = btvRat
		p.tw = tw
		p.cs = cs

		defMeanCS += cs
		if p.deflectorPercent > 0 && !p.isSink {
			meanCS += cs
		} else {
			numNoDefl++
		}
		if cs > maxCS {
			maxCS = cs
		}
		if cs < minCS {
			minCS = cs
		}
	}
	if numPlayers > numNoDefl {
		meanCS /= float64(numPlayers - numNoDefl)
	} else {
		meanCS = defMeanCS / float64(numPlayers)
	}

	results.MeanCS = meanCS
	results.MaxCS = maxCS
	results.MinCS = minCS

	beforeDrop, afterDrop := srGetDeflectorDropPerc(players, results.TotalDeflector, results.SIABActive)
	results.UnusedDeflectorBeforeSwap = beforeDrop
	results.UnusedDeflector = afterDrop

	results.Players = make([]SRSandboxPlayerResult, 0, len(players))
	for _, p := range players {
		sinkTokens := 0
		if p.isSink {
			sinkTokens = 2
		} else if cfg.TokenToggle {
			sinkTokens = 10
		}
		if numPlayers == 1 {
			sinkTokens = 0
		}

		pr := SRSandboxPlayerResult{
			Name:              p.name,
			TE:                p.te,
			LayRate:           p.layRate,
			ShipRate:          p.shipRate,
			DeliveryRate:      p.deliveryRate,
			NumTach:           p.numTach,
			NumQuant:          p.numQuant,
			Chickens:          p.chickens,
			MaxChickens:       p.maxChickens,
			ContributionRatio: p.contributionRatio,
			TimeToBoost:       p.timeToBoost,
			BoostingTime:      p.boostingTime,
			BTVRatio:          p.btvRat,
			Teamwork:          p.tw,
			CS:                p.cs,
			DeflectorPercent:  p.deflectorPercent,
			IsSink:            p.isSink,
			IsSPSwap:          p.spSwap,
			SIABActive:        p.siabActive,
			Swapped:           p.swapped,
			SwapTime:          p.swapTime,
			SinkCR:            srCalcSinkCR(numPlayers, durDays),
			SinkTokens:        sinkTokens,
		}
		if p.beforeSwap != nil {
			pr.HasBeforeSwap = true
			pr.BeforeLayRate = p.beforeSwap.layRate
			pr.BeforeShipRate = p.beforeSwap.shipRate
			pr.BeforeDeliveryRate = p.beforeSwap.deliveryRate
			pr.BeforeNumTach = p.beforeSwap.numTach
			pr.BeforeNumQuant = p.beforeSwap.numQuant
			pr.BeforeChickens = p.beforeSwap.chickens
		}
		results.Players = append(results.Players, pr)
	}

	return results
}

// FormatSRSandboxDuration formats seconds into the exact string format used by scripts.js secondsToString().
func FormatSRSandboxDuration(sec float64) string {
	if math.IsNaN(sec) || math.IsInf(sec, 0) || sec < 0 {
		return "0s"
	}
	days := int(math.Floor(sec / 3600.0 / 24.0))
	rem := math.Mod(sec, 3600.0*24.0)
	hours := int(math.Floor(rem / 3600.0))
	rem = math.Mod(rem, 3600.0)
	minutes := int(math.Floor(rem / 60.0))
	seconds := math.Mod(rem, 60.0)

	var b strings.Builder
	if days > 0 {
		if days > 1 {
			fmt.Fprintf(&b, "%ddays, ", days)
		} else {
			fmt.Fprintf(&b, "%dday, ", days)
		}
	}
	if hours > 0 {
		if hours > 1 {
			fmt.Fprintf(&b, "%dhrs, ", hours)
		} else {
			fmt.Fprintf(&b, "%dhr, ", hours)
		}
	}
	if minutes > 0 {
		fmt.Fprintf(&b, "%dmin, ", minutes)
	}
	fmt.Fprintf(&b, "%ds", int(math.Round(seconds)))
	return b.String()
}

// FormatSRSandboxComma formats a number with commas matching scripts.js commafy().
func FormatSRSandboxComma(num float64) string {
	s := strconv.FormatFloat(num, 'f', -1, 64)
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	if len(intPart) >= 5 {
		var out strings.Builder
		neg := false
		if strings.HasPrefix(intPart, "-") {
			neg = true
			intPart = intPart[1:]
		}
		if neg {
			out.WriteByte('-')
		}
		rem := len(intPart) % 3
		if rem > 0 {
			out.WriteString(intPart[:rem])
		}
		for i := rem; i < len(intPart); i += 3 {
			if i > 0 {
				out.WriteByte(',')
			}
			out.WriteString(intPart[i : i+3])
		}
		intPart = out.String()
	}
	if len(parts) == 2 {
		return intPart + "." + parts[1]
	}
	return intPart
}

func srFormatRateQHr(rate float64) string {
	val := math.Round(rate/1e9) / 1e6
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func srFormatRounded(val float64, decimals int) string {
	pow := math.Pow(10, float64(decimals))
	rounded := math.Round(val*pow) / pow
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

// SRSandboxTableColumn defines a column in an SR Sandbox results table.
type SRSandboxTableColumn struct {
	Label string
	Align StringAlign
}

// SRSandboxTableLine defines a single styled line inside a table cell.
type SRSandboxTableLine struct {
	Text  string
	Color string // "", "green", "muted", "amber", "red"
}

// SRSandboxTableCell defines a potentially multi-line cell.
type SRSandboxTableCell struct {
	Lines []SRSandboxTableLine
}

// SRSandboxTableRow defines a row of cells with optional row highlight.
type SRSandboxTableRow struct {
	Cells     []SRSandboxTableCell
	Highlight string // "", "amber"
}

// SRSandboxTableData holds the formatted Coop Summary and Player Results tables.
type SRSandboxTableData struct {
	Title         string
	Success       bool
	CoopColumns   []SRSandboxTableColumn
	CoopRows      []SRSandboxTableRow
	SIABNote      string
	PlayerColumns []SRSandboxTableColumn
	PlayerRows    []SRSandboxTableRow
	FooterNote    string
}

func srCell(text string, colorName ...string) SRSandboxTableCell {
	c := ""
	if len(colorName) > 0 {
		c = colorName[0]
	}
	return SRSandboxTableCell{
		Lines: []SRSandboxTableLine{{Text: text, Color: c}},
	}
}

func srRateLines(rate, otherRate float64, count int, stoneLabel string) []SRSandboxTableLine {
	c := ""
	if rate <= otherRate {
		c = "green"
	}
	return []SRSandboxTableLine{
		{Text: srFormatRateQHr(rate), Color: c},
		{Text: fmt.Sprintf("%d %s", count, stoneLabel), Color: "muted"},
	}
}

// BuildSRSandboxTableData formats SRSandboxResults into structured table data matching
// the Results section (coopTable + playersTable) of https://srsandbox-staabmia.netlify.app/.
func BuildSRSandboxTableData(results SRSandboxResults) SRSandboxTableData {
	title := "SR Sandbox Results"
	if results.ContractName != "" {
		title = "SR Sandbox Results — " + results.ContractName
	}

	coopCols := []SRSandboxTableColumn{
		{Label: "Production Rate (q/hr)", Align: StringAlignCenter},
		{Label: "Completion Time", Align: StringAlignCenter},
		{Label: "max CS", Align: StringAlignCenter},
		{Label: "mean CS", Align: StringAlignCenter},
		{Label: "Deflector Boost", Align: StringAlignCenter},
		{Label: "Unused Deflector %", Align: StringAlignCenter},
	}

	playerCols := []SRSandboxTableColumn{
		{Label: "Player", Align: StringAlignLeft},
		{Label: "TE", Align: StringAlignCenter},
		{Label: "elr (q/hr)", Align: StringAlignCenter},
		{Label: "sr (q/hr)", Align: StringAlignCenter},
		{Label: "Population", Align: StringAlignRight},
		{Label: "Contr. Ratio", Align: StringAlignCenter},
		{Label: "Pre-Boost / Boosting", Align: StringAlignCenter},
		{Label: "BTV / complTime", Align: StringAlignCenter},
		{Label: "Teamwork", Align: StringAlignCenter},
		{Label: "CS", Align: StringAlignRight},
	}

	data := SRSandboxTableData{
		Title:         title,
		Success:       results.Success,
		CoopColumns:   coopCols,
		PlayerColumns: playerCols,
		FooterNote:    "Simulation based on staabmia's Speedrun Sandbox (https://srsandbox-staabmia.netlify.app/)",
	}

	if !results.Success {
		data.PlayerRows = []SRSandboxTableRow{
			{
				Cells: []SRSandboxTableCell{
					srCell(""),
					srCell("Mission", "red"),
					srCell("failed,", "red"),
					srCell("we'll", "red"),
					srCell("get", "red"),
					srCell("'em", "red"),
					srCell("next", "red"),
					srCell("time!", "red"),
					srCell(""),
					srCell(""),
				},
			},
		}
		return data
	}

	// Coop Summary Row
	siabSwappedBeforeEnd := results.SIABActive && math.Round(results.SIABSwapTime) < math.Round(results.CompletionTime)
	var prodCell SRSandboxTableCell
	if siabSwappedBeforeEnd {
		prodCell = SRSandboxTableCell{
			Lines: []SRSandboxTableLine{
				{Text: srFormatRateQHr(results.CoopRateBeforeSwap)},
				{Text: srFormatRateQHr(results.CoopRate)},
			},
		}
	} else {
		prodCell = srCell(srFormatRateQHr(results.CoopRate))
	}

	var unusedCell SRSandboxTableCell
	if siabSwappedBeforeEnd {
		unusedCell = SRSandboxTableCell{
			Lines: []SRSandboxTableLine{
				{Text: fmt.Sprintf("%.0f%%", results.UnusedDeflector)},
				{Text: fmt.Sprintf("%.0f%%", results.UnusedDeflectorBeforeSwap)},
			},
		}
	} else {
		unusedCell = srCell(fmt.Sprintf("%.0f%%", results.UnusedDeflector))
	}

	data.CoopRows = []SRSandboxTableRow{
		{
			Cells: []SRSandboxTableCell{
				prodCell,
				srCell(FormatSRSandboxDuration(results.CompletionTime)),
				srCell(fmt.Sprintf("%.0f", results.MaxCS)),
				srCell(srFormatRounded(results.MeanCS, 2)),
				srCell(fmt.Sprintf("%.0f%%", results.TotalDeflector)),
				unusedCell,
			},
		},
	}

	if results.SIABActive {
		if results.SIABSwapTime < results.CompletionTime-1 {
			data.SIABNote = fmt.Sprintf("SIAB swapped after: %s", FormatSRSandboxDuration(results.SIABSwapTime))
		} else {
			data.SIABNote = "SIAB active entire contract."
		}
	}

	// Player Rows
	data.PlayerRows = make([]SRSandboxTableRow, 0, len(results.Players))
	for _, p := range results.Players {
		nameLines := []SRSandboxTableLine{{Text: UnescapeDiscordMarkdown(p.Name)}}
		if p.IsSPSwap {
			nameLines = append(nameLines, SRSandboxTableLine{Text: "[SP]", Color: "amber"})
		}
		if p.IsSink && !results.New2p0 {
			nameLines = append(nameLines, SRSandboxTableLine{
				Text:  fmt.Sprintf("Sink: %dCR, %dT", p.SinkCR, p.SinkTokens),
				Color: "muted",
			})
		}

		useSIAB := results.SIABActive && p.SIABActive && p.HasBeforeSwap && results.SIABSwapTime < results.CompletionTime-1
		var elrLines, srLines, popLines []SRSandboxTableLine

		if useSIAB {
			elrLines = append(
				srRateLines(p.BeforeLayRate, p.BeforeShipRate, p.BeforeNumTach, "Tach"),
				srRateLines(p.LayRate, p.ShipRate, p.NumTach, "Tach")...,
			)
			srLines = append(
				srRateLines(p.BeforeShipRate, p.BeforeLayRate, p.BeforeNumQuant, "Quant"),
				srRateLines(p.ShipRate, p.LayRate, p.NumQuant, "Quant")...,
			)
			popLines = []SRSandboxTableLine{
				{Text: FormatSRSandboxComma(p.BeforeChickens)},
				{Text: FormatSRSandboxComma(p.Chickens)},
			}
		} else {
			layRate, shipRate := p.LayRate, p.ShipRate
			numTach, numQuant := p.NumTach, p.NumQuant
			chickens := p.Chickens
			if results.SIABActive && p.HasBeforeSwap {
				layRate, shipRate = p.BeforeLayRate, p.BeforeShipRate
				numTach, numQuant = p.BeforeNumTach, p.BeforeNumQuant
				chickens = p.BeforeChickens
			}
			elrLines = srRateLines(layRate, shipRate, numTach, "Tach")
			srLines = srRateLines(shipRate, layRate, numQuant, "Quant")
			popLines = []SRSandboxTableLine{{Text: FormatSRSandboxComma(chickens)}}
		}

		if p.Swapped && p.MaxChickens > 0 && p.Chickens < p.MaxChickens*(1.0-1e-9) {
			pct := math.Round(p.Chickens/p.MaxChickens*1000.0) / 10.0
			popLines = append(popLines, SRSandboxTableLine{
				Text:  fmt.Sprintf("%.1f%% habs", pct),
				Color: "muted",
			})
		}

		preBoostStr := "N/A"
		if p.TimeToBoost != results.Duration {
			preBoostStr = FormatSRSandboxDuration(p.TimeToBoost)
		}
		timeCell := SRSandboxTableCell{
			Lines: []SRSandboxTableLine{
				{Text: preBoostStr},
				{Text: FormatSRSandboxDuration(p.BoostingTime), Color: "muted"},
			},
		}

		highlight := ""
		if p.IsSPSwap {
			highlight = "amber"
		}

		data.PlayerRows = append(data.PlayerRows, SRSandboxTableRow{
			Highlight: highlight,
			Cells: []SRSandboxTableCell{
				{Lines: nameLines},
				srCell(strconv.Itoa(p.TE)),
				{Lines: elrLines},
				{Lines: srLines},
				{Lines: popLines},
				srCell(srFormatRounded(p.ContributionRatio, 3)),
				timeCell,
				srCell(srFormatRounded(p.BTVRatio, 3)),
				srCell(srFormatRounded(p.Teamwork, 6)),
				srCell(fmt.Sprintf("%.0f", p.CS)),
			},
		})
	}

	return data
}

type srLoadedFont struct {
	face     font.Face
	sfntFont *sfnt.Font
}

func srLoadFont(fontSize float64) srLoadedFont {
	candidates := []string{
		filepath.Join(config.BannerPath, "Always Together.otf"),
		"./banners/Always Together.otf",
		"./emoji/Always Together.otf",
		"../banners/Always Together.otf",
		"../../banners/Always Together.otf",
		"../emoji/Always Together.otf",
		"../../emoji/Always Together.otf",
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		sfFont, err := sfnt.Parse(data)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(sfFont, &opentype.FaceOptions{
			Size:    fontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil {
			return srLoadedFont{face: face, sfntFont: sfFont}
		}
	}
	return srLoadedFont{face: srLoadFallbackFont(fontSize), sfntFont: nil}
}

func srLoadFallbackFont(fontSize float64) font.Face {
	sfFont, err := opentype.Parse(goregular.TTF)
	if err == nil {
		face, err := opentype.NewFace(sfFont, &opentype.FaceOptions{
			Size:    fontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil {
			return face
		}
	}
	return basicfont.Face7x13
}

func srIsGameFontRune(lf srLoadedFont, sfBuf *sfnt.Buffer, r rune) bool {
	if lf.sfntFont == nil {
		return false
	}
	if r < 32 || r > 126 {
		return false
	}
	idx, err := lf.sfntFont.GlyphIndex(sfBuf, r)
	return err == nil && idx != 0
}

func srMeasureString(lf srLoadedFont, fallback font.Face, str string) int {
	str = ei.NormalizePlayerNameForDisplay(str)
	if lf.sfntFont == nil {
		return font.MeasureString(lf.face, str).Ceil()
	}
	var sfBuf sfnt.Buffer
	allGame := true
	for _, r := range str {
		if !srIsGameFontRune(lf, &sfBuf, r) {
			allGame = false
			break
		}
	}
	if allGame {
		return font.MeasureString(lf.face, str).Ceil()
	}
	totalW := 0
	for _, r := range str {
		if srIsGameFontRune(lf, &sfBuf, r) {
			totalW += font.MeasureString(lf.face, string(r)).Ceil()
		} else {
			totalW += font.MeasureString(fallback, string(r)).Ceil()
		}
	}
	return totalW
}

func srDrawString(img *image.RGBA, lf srLoadedFont, fallback font.Face, str string, startX, drawY int, c color.Color) {
	str = ei.NormalizePlayerNameForDisplay(str)
	if lf.sfntFont == nil {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: lf.face,
			Dot:  fixed.Point26_6{X: fixed.I(startX), Y: fixed.I(drawY)},
		}
		d.DrawString(str)
		return
	}
	var sfBuf sfnt.Buffer
	allGame := true
	for _, r := range str {
		if !srIsGameFontRune(lf, &sfBuf, r) {
			allGame = false
			break
		}
	}
	if allGame {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: lf.face,
			Dot:  fixed.Point26_6{X: fixed.I(startX), Y: fixed.I(drawY)},
		}
		d.DrawString(str)
		return
	}
	curX := startX
	for _, r := range str {
		rStr := string(r)
		f := fallback
		if srIsGameFontRune(lf, &sfBuf, r) {
			f = lf.face
		}
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: f,
			Dot:  fixed.Point26_6{X: fixed.I(curX), Y: fixed.I(drawY)},
		}
		d.DrawString(rStr)
		curX += font.MeasureString(f, rStr).Ceil()
	}
}

type srTableLayout struct {
	colWidths  []int
	colX       []int
	rowHeights []int
	headerH    int
	totalW     int
	totalH     int
}

func srMeasureTable(cols []SRSandboxTableColumn, rows []SRSandboxTableRow, lf srLoadedFont, fallback font.Face, lineH, cellPadX, cellPadY int) srTableLayout {
	colWidths := make([]int, len(cols))
	for i, col := range cols {
		maxW := srMeasureString(lf, fallback, col.Label)
		for _, row := range rows {
			if i < len(row.Cells) {
				for _, line := range row.Cells[i].Lines {
					if w := srMeasureString(lf, fallback, line.Text); w > maxW {
						maxW = w
					}
				}
			}
		}
		colWidths[i] = maxW + cellPadX*2
	}

	colX := make([]int, len(cols)+1)
	for i := range cols {
		colX[i+1] = colX[i] + colWidths[i]
	}

	headerH := lineH + cellPadY*2
	rowHeights := make([]int, len(rows))
	totalRowsH := 0
	for rIdx, row := range rows {
		maxLines := 1
		for _, cell := range row.Cells {
			if len(cell.Lines) > maxLines {
				maxLines = len(cell.Lines)
			}
		}
		rowH := maxLines*lineH + cellPadY*2
		rowHeights[rIdx] = rowH
		totalRowsH += rowH
	}

	return srTableLayout{
		colWidths:  colWidths,
		colX:       colX,
		rowHeights: rowHeights,
		headerH:    headerH,
		totalW:     colX[len(cols)],
		totalH:     headerH + 1 + totalRowsH,
	}
}

// RenderSRSandboxTableImage renders SRSandboxTableData (Coop Summary + Player Results) as a PNG image.
func RenderSRSandboxTableImage(data SRSandboxTableData) ([]byte, error) {
	titleFont := srLoadFont(24.0)
	titleFallback := srLoadFallbackFont(21.0)
	bodyFont := srLoadFont(20.0)
	bodyFallback := srLoadFallbackFont(18.0)
	footerFont := srLoadFont(15.0)
	footerFallback := srLoadFallbackFont(14.0)

	titleMetrics := titleFont.face.Metrics()
	titleAscent := titleMetrics.Ascent.Ceil()
	titleDescent := titleMetrics.Descent.Ceil()

	bodyMetrics := bodyFont.face.Metrics()
	bodyAscent := bodyMetrics.Ascent.Ceil()
	bodyDescent := bodyMetrics.Descent.Ceil()
	lineH := max(22, bodyAscent+bodyDescent+4)

	footerMetrics := footerFont.face.Metrics()
	footerAscent := footerMetrics.Ascent.Ceil()
	footerDescent := footerMetrics.Descent.Ceil()

	const (
		padX       = 18
		padY       = 16
		cellPadX   = 10
		cellPadY   = 6
		sectionGap = 14
	)

	coopLayout := srMeasureTable(data.CoopColumns, data.CoopRows, bodyFont, bodyFallback, lineH, cellPadX, cellPadY)
	playerLayout := srMeasureTable(data.PlayerColumns, data.PlayerRows, bodyFont, bodyFallback, lineH, cellPadX, cellPadY)

	titleW := srMeasureString(titleFont, titleFallback, data.Title)
	footerW := 0
	if data.FooterNote != "" {
		footerW = srMeasureString(footerFont, footerFallback, data.FooterNote)
	}
	contentW := max(max(titleW, footerW), max(coopLayout.totalW, playerLayout.totalW))

	// Stretch coop table columns proportionally if narrower than contentW so both tables align neatly.
	if len(coopLayout.colWidths) > 0 && coopLayout.totalW > 0 && coopLayout.totalW < contentW {
		extra := contentW - coopLayout.totalW
		perCol := extra / len(coopLayout.colWidths)
		rem := extra % len(coopLayout.colWidths)
		for i := range coopLayout.colWidths {
			coopLayout.colWidths[i] += perCol
			if i < rem {
				coopLayout.colWidths[i]++
			}
			coopLayout.colX[i+1] = coopLayout.colX[i] + coopLayout.colWidths[i]
		}
		coopLayout.totalW = contentW
	}

	titleH := titleAscent + titleDescent + 8
	siabH := 0
	if data.SIABNote != "" {
		siabH = lineH + 6
	}
	footerH := 0
	if data.FooterNote != "" {
		footerH = footerAscent + footerDescent + 10
	}

	imgW := contentW + padX*2
	imgH := padY + titleH + sectionGap
	if len(data.CoopRows) > 0 {
		imgH += coopLayout.totalH + sectionGap
	}
	if siabH > 0 {
		imgH += siabH + 6
	}
	imgH += playerLayout.totalH + footerH + padY

	bgColor := color.RGBA{R: 0x1e, G: 0x1f, B: 0x22, A: 255}
	headerBgColor := color.RGBA{R: 0x2b, G: 0x2d, B: 0x31, A: 255}
	altRowColor := color.RGBA{R: 0x23, G: 0x25, B: 0x29, A: 255}
	amberRowColor := color.RGBA{R: 0x33, G: 0x2a, B: 0x1c, A: 255}
	dividerColor := color.RGBA{R: 0x4e, G: 0x50, B: 0x58, A: 255}
	rowLineColor := color.RGBA{R: 0x35, G: 0x37, B: 0x3c, A: 255}
	whiteColor := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 255}

	getColor := func(name string) color.RGBA {
		switch name {
		case "green":
			return color.RGBA{R: 0x57, G: 0xf2, B: 0x87, A: 255}
		case "red":
			return color.RGBA{R: 0xed, G: 0x42, B: 0x45, A: 255}
		case "amber":
			return color.RGBA{R: 0xf5, G: 0x9e, B: 0x0b, A: 255}
		case "muted":
			return color.RGBA{R: 0xb5, G: 0xba, B: 0xc1, A: 255}
		default:
			return whiteColor
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	drawHLine := func(y, x1, x2 int, c color.RGBA) {
		for x := x1; x < x2; x++ {
			img.Set(x, y, c)
		}
	}

	drawTable := func(startY int, cols []SRSandboxTableColumn, rows []SRSandboxTableRow, layout srTableLayout) int {
		// Header background
		headerRect := image.Rect(padX, startY, padX+layout.totalW, startY+layout.headerH)
		draw.Draw(img, headerRect, &image.Uniform{headerBgColor}, image.Point{}, draw.Src)

		drawAligned := func(text string, colIdx, drawY int, c color.Color) {
			tw := srMeasureString(bodyFont, bodyFallback, text)
			xStart := padX + layout.colX[colIdx]
			xEnd := padX + layout.colX[colIdx+1]
			w := layout.colWidths[colIdx]
			var drawX int
			switch cols[colIdx].Align {
			case StringAlignLeft:
				drawX = xStart + cellPadX
			case StringAlignRight, StringAlignCenterRight:
				drawX = xEnd - cellPadX - tw
			default:
				drawX = xStart + (w-tw)/2
			}
			srDrawString(img, bodyFont, bodyFallback, text, drawX, drawY, c)
		}

		headerBaseline := startY + cellPadY + bodyAscent
		for i, col := range cols {
			drawAligned(col.Label, i, headerBaseline, whiteColor)
		}

		curY := startY + layout.headerH
		drawHLine(curY, padX, padX+layout.totalW, dividerColor)
		curY++

		for rIdx, row := range rows {
			rowH := layout.rowHeights[rIdx]
			rowRect := image.Rect(padX, curY, padX+layout.totalW, curY+rowH)
			if row.Highlight == "amber" {
				draw.Draw(img, rowRect, &image.Uniform{amberRowColor}, image.Point{}, draw.Src)
			} else if rIdx%2 == 1 {
				draw.Draw(img, rowRect, &image.Uniform{altRowColor}, image.Point{}, draw.Src)
			}

			for cIdx, cell := range row.Cells {
				if cIdx >= len(cols) {
					break
				}
				numLines := len(cell.Lines)
				if numLines == 0 {
					continue
				}
				blockH := numLines * lineH
				topOffset := (rowH - blockH) / 2
				for lIdx, line := range cell.Lines {
					baselineY := curY + topOffset + lIdx*lineH + bodyAscent
					drawAligned(line.Text, cIdx, baselineY, getColor(line.Color))
				}
			}

			curY += rowH
			if rIdx < len(rows)-1 {
				drawHLine(curY-1, padX, padX+layout.totalW, rowLineColor)
			}
		}

		return curY
	}

	// Title
	curY := padY
	srDrawString(img, titleFont, titleFallback, data.Title, padX, curY+titleAscent, whiteColor)
	curY += titleH + sectionGap

	// Coop Summary Table
	if len(data.CoopRows) > 0 {
		curY = drawTable(curY, data.CoopColumns, data.CoopRows, coopLayout)
		curY += sectionGap
	}

	// Optional SIAB Swap Note
	if data.SIABNote != "" {
		srDrawString(img, bodyFont, bodyFallback, data.SIABNote, padX, curY+bodyAscent, getColor("amber"))
		curY += siabH + 6
	}

	// Players Table
	curY = drawTable(curY, data.PlayerColumns, data.PlayerRows, playerLayout)

	// Footer citation
	if data.FooterNote != "" {
		curY += 10
		srDrawString(img, footerFont, footerFallback, data.FooterNote, padX, curY+footerAscent, getColor("muted"))
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateSRSandboxResultsImage runs the SR Sandbox simulation, builds the Results table data,
// and renders it to a PNG image.
func GenerateSRSandboxResultsImage(cfg SRSandboxConfig) (SRSandboxResults, SRSandboxTableData, []byte, error) {
	results := RunSRSandboxSimulation(cfg)
	tableData := BuildSRSandboxTableData(results)
	imgBytes, err := RenderSRSandboxTableImage(tableData)
	if err != nil {
		return results, tableData, nil, err
	}
	return results, tableData, imgBytes, nil
}
