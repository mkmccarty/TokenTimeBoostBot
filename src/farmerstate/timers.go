package farmerstate

import (
	"log"
	"time"
)

// GetAllTimers returns all timers from the database.
func GetAllTimers() []Timer {
	FlushPendingSaves()
	timers, err := queries.GetTimers(ctx)
	if err != nil {
		log.Println("GetAllTimers error:", err)
		return nil
	}
	return timers
}

// AddTimer inserts a new timer into the database.
func AddTimer(id, userID, channelID, msgID string, reminder time.Time, message string, duration int64, origChannelID, origMsgID string, active bool) {
	FlushPendingSaves()
	err := queries.InsertTimer(ctx, InsertTimerParams{
		ID:                id,
		UserID:            userID,
		ChannelID:         channelID,
		MsgID:             msgID,
		Reminder:          reminder,
		Message:           message,
		Duration:          duration,
		OriginalChannelID: origChannelID,
		OriginalMsgID:     origMsgID,
		Active:            active,
	})
	if err != nil {
		log.Println("AddTimer error:", err)
	}
}

// UpdateTimerState updates the active state of a timer in the database.
func UpdateTimerState(id string, active bool) {
	FlushPendingSaves()
	err := queries.UpdateTimerState(ctx, UpdateTimerStateParams{
		ID:     id,
		Active: active,
	})
	if err != nil {
		log.Println("UpdateTimerState error:", err)
	}
}

// UpdateTimerMsg updates the channel and message ID of a timer in the database.
func UpdateTimerMsg(id, channelID, msgID string) {
	FlushPendingSaves()
	err := queries.UpdateTimerMsg(ctx, UpdateTimerMsgParams{
		ID:        id,
		ChannelID: channelID,
		MsgID:     msgID,
	})
	if err != nil {
		log.Println("UpdateTimerMsg error:", err)
	}
}

// DeleteTimer removes a timer from the database by its ID.
func DeleteTimer(id string) {
	FlushPendingSaves()
	err := queries.DeleteTimer(ctx, id)
	if err != nil {
		log.Println("DeleteTimer error:", err)
	}
}

// DeleteInactiveTimers removes all inactive timers from the database.
func DeleteInactiveTimers() {
	FlushPendingSaves()
	err := queries.DeleteInactiveTimers(ctx)
	if err != nil {
		log.Println("DeleteInactiveTimers error:", err)
	}
}
