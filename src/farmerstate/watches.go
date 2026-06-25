package farmerstate

import (
	"log"
)

// AddWatch inserts a new watch into the database.
func AddWatch(userID, watchType, targetID string) {
	FlushPendingSaves()
	err := queries.InsertWatch(ctx, InsertWatchParams{
		UserID:    userID,
		WatchType: watchType,
		TargetID:  targetID,
	})
	if err != nil {
		log.Println("AddWatch error:", err)
	}
}

// GetWatchesForUser returns all watches for a given user.
func GetWatchesForUser(userID string) []Watch {
	FlushPendingSaves()
	watches, err := queries.GetWatchesForUser(ctx, userID)
	if err != nil {
		log.Println("GetWatchesForUser error:", err)
		return nil
	}
	return watches
}

// GetAllWatches returns all watches in the database.
func GetAllWatches() []Watch {
	FlushPendingSaves()
	watches, err := queries.GetAllWatches(ctx)
	if err != nil {
		log.Println("GetAllWatches error:", err)
		return nil
	}
	return watches
}

// DeleteWatch deletes a specific watch.
func DeleteWatch(userID, watchType, targetID string) {
	FlushPendingSaves()
	err := queries.DeleteWatch(ctx, DeleteWatchParams{
		UserID:    userID,
		WatchType: watchType,
		TargetID:  targetID,
	})
	if err != nil {
		log.Println("DeleteWatch error:", err)
	}
}

// DeleteUserWatches deletes all watches for a given user.
func DeleteUserWatches(userID string) {
	FlushPendingSaves()
	err := queries.DeleteUserWatches(ctx, userID)
	if err != nil {
		log.Println("DeleteUserWatches error:", err)
	}
}
