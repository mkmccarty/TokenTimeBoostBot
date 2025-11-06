package ei

import "time"

var currentGGEvent = 1.0
var currentUltraGGEvent = 1.0
var currentEventEndsGG time.Time
var currentEarningsEvent = 1.0
var currentEarningsEventUltra = 1.0

var currentResearchDiscountEvent = 1.0

//var currentResearchDiscountEventUltra = 1.0

// GetGenerousGiftEvent will return the current Generous Gift event multiplier
func GetGenerousGiftEvent() (float64, float64, time.Time) {
	return currentGGEvent, currentUltraGGEvent, currentEventEndsGG
}

// SetGenerousGiftEvent will return the current Generous Gift event multiplier
func SetGenerousGiftEvent(gg float64, ugg float64, endtime time.Time) {
	currentGGEvent = gg
	currentUltraGGEvent = ugg
	currentEventEndsGG = endtime
}

// SetEarningsEvent will set the current earnings event multipliers
func SetEarningsEvent(earnings float64, ultraEarnings float64) {
	currentEarningsEvent = earnings
	currentEarningsEventUltra = ultraEarnings
}

// SetResearchDiscountEvent will set the current research discount event multipliers
func SetResearchDiscountEvent(discount float64) {
	currentResearchDiscountEvent = discount
	//currentResearchDiscountEventUltra = ultraDiscount
}
