package dashboard

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc/dctest"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
	_ "modernc.org/sqlite"
)

func TestTimer_ExpirationAndAutoDelete_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := dctest.New()
		userID := "4"
		farmerstate.SetMiscSettingString(userID, "timer_dm_timeout", "10s")

		cmd := dctest.CommandEvent("timer",
			dctest.StringOption{Name: "duration", Value: "15m"},
			dctest.StringOption{Name: "message", Value: "Boost ready!"},
		)

		HandleTimer(client, cmd)

		// Verification 1: Timer was created, not expired yet
		if client.Called("SendMessage") {
			t.Fatalf("SendMessage should not be called before timer expires")
		}

		// Fast-forward fake time by 15 minutes to trigger the timer
		time.Sleep(15 * time.Minute)
		synctest.Wait()

		// Verification 2: DM message was sent
		if !client.Called("CreateUserChannel") {
			t.Errorf("expected CreateUserChannel call when timer expires")
		}
		if !client.Called("SendMessage") {
			t.Fatalf("expected SendMessage call when timer expires")
		}
		sends := client.CallsTo("SendMessage")
		if len(sends) == 0 || len(sends[0].Args) < 2 {
			t.Fatalf("expected SendMessage call with message content")
		}
		content := sends[0].Args[1]
		if content == "" {
			t.Errorf("expected non-empty message content")
		}

		// Message should stay for 10s (per timer_dm_timeout setting) then delete
		if client.Called("DeleteMessage") {
			t.Fatalf("DeleteMessage called too early")
		}

		// Fast-forward fake time by 10s
		time.Sleep(10 * time.Second)
		synctest.Wait()

		// Verification 3: Message was auto-deleted
		if !client.Called("DeleteMessage") {
			t.Errorf("expected DeleteMessage call after dm-timeout elapsed")
		}
	})
}

func TestTimer_RepeatAndClose_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := dctest.New()
		userID := "4"
		// 0 duration means message stays until closed
		farmerstate.SetMiscSettingString(userID, "timer_dm_timeout", "0")

		cmd := dctest.CommandEvent("timer",
			dctest.StringOption{Name: "duration", Value: "10m"},
			dctest.StringOption{Name: "message", Value: "Collect tokens"},
		)

		HandleTimer(client, cmd)

		// Elapse 10 minutes to fire the timer
		time.Sleep(10 * time.Minute)
		synctest.Wait()

		timersMutex.Lock()
		var timerID string
		if len(timers) > 0 {
			timerID = timers[len(timers)-1].ID
		}
		timersMutex.Unlock()

		if !client.Called("SendMessage") {
			t.Fatalf("expected timer message to be sent")
		}

		// Simulate user clicking "Repeat 10m Timer" button
		repeatBtn := dctest.ComponentButtonEvent("timer_btn#repeat#" + timerID)
		HandleTimerInteraction(client, repeatBtn)

		// Verify acknowledgement message auto-deletes after 10s
		if client.Called("DeleteMessage") {
			t.Fatalf("DeleteMessage should not be called yet for acknowledgement")
		}
		time.Sleep(10 * time.Second)
		synctest.Wait()

		if !client.Called("DeleteMessage") {
			t.Errorf("expected acknowledgement message to be deleted after 10s")
		}

		// Fast-forward another 10m for the repeated timer to expire
		client.ResetCalls()
		time.Sleep(10 * time.Minute)
		synctest.Wait()

		if !client.Called("SendMessage") {
			t.Fatalf("expected repeated timer message to be sent")
		}

		timersMutex.Lock()
		var repeatedTimerID string
		if len(timers) > 0 {
			repeatedTimerID = timers[len(timers)-1].ID
		}
		timersMutex.Unlock()

		// Simulate user clicking "Close" button
		closeBtn := dctest.ComponentButtonEvent("timer_btn#close#" + repeatedTimerID)
		HandleTimerInteraction(client, closeBtn)

		if !client.Called("DeleteMessage") {
			t.Errorf("expected DeleteMessage call on Close button click")
		}
	})
}
