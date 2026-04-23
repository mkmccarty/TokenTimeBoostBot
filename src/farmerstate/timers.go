package farmerstate

import (
	"log"
	"time"
)

func GetAllTimers() []Timer {
	FlushPendingSaves()
	timers, err := queries.GetTimers(ctx)
	if err != nil {
		log.Println("GetAllTimers error:", err)
		return nil
	}
	return timers
}

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

func DeleteTimer(id string) {
	FlushPendingSaves()
	err := queries.DeleteTimer(ctx, id)
	if err != nil {
		log.Println("DeleteTimer error:", err)
	}
}

func DeleteInactiveTimers() {
	FlushPendingSaves()
	err := queries.DeleteInactiveTimers(ctx)
	if err != nil {
		log.Println("DeleteInactiveTimers error:", err)
	}
}