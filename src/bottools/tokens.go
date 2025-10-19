package bottools

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// GetTokenValue calculates the token value based on the given parameters
func GetTokenValue(seconds float64, durationSeconds float64) float64 {
	currentval := max(0.03, math.Pow(1-0.9*(min(seconds, durationSeconds)/durationSeconds), 4))

	return math.Round(currentval*1000) / 1000
}

// FutureToken represents a token with its value and time
type FutureToken struct {
	Value float64
	Time  time.Time
}

// CalculateFutureTokenLogs calculates the future token logs based on the given parameters
func CalculateFutureTokenLogs(maxEntries int, startTime time.Time, minutesPerToken int, duration time.Duration, rateSecondPerTokens float64) ([]FutureToken, []FutureToken) {
	estimatedCapacity := int(maxEntries * 2)

	futureTokenLog := make([]FutureToken, 0, estimatedCapacity)
	futureTokenLogGG := make([]FutureToken, 0, estimatedCapacity)
	//futureTokenLogTimes := make([]time.Time, 0, estimatedCapacity)
	//futureTokenLogGGTimes := make([]time.Time, 0, estimatedCapacity)

	_, _, endGG := ei.GetGenerousGiftEvent()

	endTime := startTime.Add(duration)
	endTime = endTime.Add(120 * time.Hour) // Give estimates for up to 2 hours beyond contract end
	tokenTime := time.Now()
	tokenTime = tokenTime.Add(time.Duration(rateSecondPerTokens) * time.Second)
	for tokenTime.Before(endTime) {
		val := GetTokenValue(tokenTime.Sub(startTime).Seconds(), duration.Seconds())
		futureTokenLog = append(futureTokenLog, FutureToken{Value: val, Time: tokenTime})
		//futureTokenLogTimes = append(futureTokenLogTimes, tokenTime)
		if tokenTime.Before(endGG) {
			futureTokenLogGG = append(futureTokenLogGG, FutureToken{Value: val, Time: tokenTime})
		}
		//futureTokenLogGGTimes = append(futureTokenLogGGTimes, tokenTime)
		tokenTime = tokenTime.Add(time.Duration(rateSecondPerTokens) * time.Second)
		if len(futureTokenLog) > maxEntries {
			break
		}
	}
	// Now for the timer tokens, start with next timer
	tokenTime = startTime.Add(time.Duration(minutesPerToken) * time.Minute)
	for tokenTime.Before(time.Now()) {
		tokenTime = tokenTime.Add(time.Duration(minutesPerToken) * time.Minute)
	}
	for tokenTime.Before(endTime) {
		val := GetTokenValue(tokenTime.Sub(startTime).Seconds(), duration.Seconds())
		futureTokenLog = append(futureTokenLog, FutureToken{Value: val, Time: tokenTime})
		tokenTime = tokenTime.Add(time.Duration(minutesPerToken) * time.Minute)
	}

	sort.Slice(futureTokenLog, func(i, j int) bool {
		return futureTokenLog[i].Value > futureTokenLog[j].Value
	})
	futureTokenLogGG = append(futureTokenLogGG, futureTokenLog...)
	sort.Slice(futureTokenLogGG, func(i, j int) bool {
		return futureTokenLogGG[i].Value > futureTokenLogGG[j].Value
	})

	return futureTokenLog, futureTokenLogGG
}

// CalculateTcountTtime calculates the token count and time based on the given parameters
func CalculateTcountTtime(tokenValue float64, tval float64, valueLog []FutureToken) (string, string, int) {
	tcount := "√"
	ttime := ""
	count := 0

	userTokenValue := tokenValue
	if userTokenValue < tval {
		tcount = "∞"
		for i, v := range valueLog {
			userTokenValue += v.Value
			if userTokenValue >= tval {
				tcount = fmt.Sprintf("%d", i+1)
				count = i + 1
				if i < len(valueLog) { // Ensure index is within bounds
					ttime = fmt.Sprintf("~ <t:%d:t>", valueLog[i].Time.Unix())
				}
				if count >= 99 {
					tcount = "≥99"
				}
				break
			}
		}
	}
	return tcount, ttime, count
}
