package bottools

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// GetTokenValue calculates the token value based on the given parameters
func GetTokenValue(seconds float64, durationSeconds float64) float64 {
	currentval := max(0.03, math.Pow(1-0.9*(min(seconds, durationSeconds)/durationSeconds), 4))

	return math.Round(currentval*1000) / 1000
}

// CalculateFutureTokenLogs calculates the future token logs based on the given parameters
func CalculateFutureTokenLogs(maxEntries int, startTime time.Time, minutesPerToken int, duration time.Duration, rateSecondPerTokens float64) ([]float64, []time.Time, []float64, []time.Time) {
	estimatedCapacity := int(maxEntries * 2)

	futureTokenLog := make([]float64, 0, estimatedCapacity)
	futureTokenLogGG := make([]float64, 0, estimatedCapacity)
	futureTokenLogTimes := make([]time.Time, 0, estimatedCapacity)
	futureTokenLogGGTimes := make([]time.Time, 0, estimatedCapacity)

	endTime := startTime.Add(duration)
	tokenTime := time.Now()
	for tokenTime.Before(endTime) {
		val := GetTokenValue(tokenTime.Sub(startTime).Seconds(), duration.Seconds())
		futureTokenLog = append(futureTokenLog, val)
		futureTokenLogTimes = append(futureTokenLogTimes, tokenTime)
		futureTokenLogGG = append(futureTokenLogGG, val)
		futureTokenLogGGTimes = append(futureTokenLogGGTimes, tokenTime)
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
		futureTokenLog = append(futureTokenLog, val)
		futureTokenLogTimes = append(futureTokenLogTimes, tokenTime)
		tokenTime = tokenTime.Add(time.Duration(minutesPerToken) * time.Minute)
	}

	sort.Slice(futureTokenLog, func(i, j int) bool {
		return futureTokenLog[i] > futureTokenLog[j]
	})
	sort.Slice(futureTokenLogTimes, func(i, j int) bool {
		return futureTokenLogTimes[i].Before(futureTokenLogTimes[j])
	})
	futureTokenLogGG = append(futureTokenLogGG, futureTokenLog...)
	futureTokenLogGGTimes = append(futureTokenLogGGTimes, futureTokenLogTimes...)
	sort.Slice(futureTokenLogGG, func(i, j int) bool {
		return futureTokenLogGG[i] > futureTokenLogGG[j]
	})
	sort.Slice(futureTokenLogGGTimes, func(i, j int) bool {
		return futureTokenLogGGTimes[i].Before(futureTokenLogGGTimes[j])
	})

	return futureTokenLog, futureTokenLogTimes, futureTokenLogGG, futureTokenLogGGTimes
}

// CalculateTcountTtime calculates the token count and time based on the given parameters
func CalculateTcountTtime(tokenValue float64, tval float64, valueLog []float64, valueTime []time.Time) (string, string, int) {
	tcount := "√"
	ttime := ""
	count := 0

	uTval := tokenValue
	if uTval < tval {
		tcount = "∞"
		for i, v := range valueLog {
			uTval += v
			if uTval >= tval {
				tcount = fmt.Sprintf("%d", i+1)
				count = i + 1
				if i < len(valueTime) { // Ensure index is within bounds
					ttime = fmt.Sprintf("<t:%d:R>", valueTime[i].Unix())
				}
				break
			}
		}
	}
	return tcount, ttime, count
}
