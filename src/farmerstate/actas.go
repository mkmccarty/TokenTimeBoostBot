package farmerstate

import (
	"log"
	"sync"
	"time"
)

type pendingActAsRegistration struct {
	MainUserID string
	AltUserID  string
	Code       string
	ExpiresAt  time.Time
}

var (
	pendingActAsRegistrations = make(map[string]pendingActAsRegistration)
	pendingActAsMutex         sync.RWMutex
)

// StorePendingActAsRegistration saves a pending verification code for account linking.
func StorePendingActAsRegistration(mainUserID, altUserID, code string, ttl time.Duration) {
	pendingActAsMutex.Lock()
	defer pendingActAsMutex.Unlock()
	pendingActAsRegistrations[mainUserID] = pendingActAsRegistration{
		MainUserID: mainUserID,
		AltUserID:  altUserID,
		Code:       code,
		ExpiresAt:  time.Now().Add(ttl),
	}
}

// GetPendingActAsRegistration retrieves a pending registration if not expired.
func GetPendingActAsRegistration(mainUserID string) (pendingActAsRegistration, bool) {
	pendingActAsMutex.RLock()
	defer pendingActAsMutex.RUnlock()
	p, ok := pendingActAsRegistrations[mainUserID]
	if !ok || time.Now().After(p.ExpiresAt) {
		return pendingActAsRegistration{}, false
	}
	return p, true
}

// ClearPendingActAsRegistration removes a pending registration entry.
func ClearPendingActAsRegistration(mainUserID string) {
	pendingActAsMutex.Lock()
	defer pendingActAsMutex.Unlock()
	delete(pendingActAsRegistrations, mainUserID)
}

// AddActAsLink links altUserID as an alternate account for mainUserID.
func AddActAsLink(mainUserID, altUserID string) error {
	FlushPendingSaves()
	return queries.InsertActAsLink(ctx, InsertActAsLinkParams{
		MainUserID: mainUserID,
		AltUserID:  altUserID,
	})
}

// RemoveActAsLink removes the link between mainUserID and altUserID.
func RemoveActAsLink(mainUserID, altUserID string) error {
	FlushPendingSaves()
	_, err := queries.DeleteActAsLink(ctx, DeleteActAsLinkParams{
		MainUserID:   mainUserID,
		AltUserID:    altUserID,
		MainUserID_2: altUserID,
		AltUserID_2:  mainUserID,
	})
	if err == nil {
		_, _ = queries.DeleteActAsSwitchForPair(ctx, DeleteActAsSwitchForPairParams{
			MainUserID:   mainUserID,
			AltUserID:    altUserID,
			MainUserID_2: altUserID,
			AltUserID_2:  mainUserID,
		})
	}
	return err
}

// DeleteAllActAsLinks removes all links involving userID.
func DeleteAllActAsLinks(userID string) error {
	FlushPendingSaves()
	_, err := queries.DeleteAllActAsLinksForUser(ctx, DeleteAllActAsLinksForUserParams{
		MainUserID: userID,
		AltUserID:  userID,
	})
	if err == nil {
		_, _ = queries.DeleteAllActAsSwitchesForUser(ctx, DeleteAllActAsSwitchesForUserParams{
			MainUserID: userID,
			AltUserID:  userID,
		})
	}
	return err
}

// GetActAsLinks returns all alternate account user IDs linked to mainUserID.
func GetActAsLinks(mainUserID string) []string {
	FlushPendingSaves()
	links, err := queries.GetActAsLinksForMain(ctx, mainUserID)
	if err != nil {
		log.Println("GetActAsLinks error:", err)
		return nil
	}
	return links
}

// GetActAsMains returns all main user IDs that have linked altUserID as an alternate.
func GetActAsMains(altUserID string) []string {
	FlushPendingSaves()
	mains, err := queries.GetActAsMainsForAlt(ctx, altUserID)
	if err != nil {
		log.Println("GetActAsMains error:", err)
		return nil
	}
	return mains
}

// GetAllActAsLinkedUsers returns all user IDs linked with userID in either direction.
func GetAllActAsLinkedUsers(userID string) []string {
	FlushPendingSaves()
	seen := make(map[string]bool)
	var res []string
	for _, id := range GetActAsLinks(userID) {
		if !seen[id] {
			seen[id] = true
			res = append(res, id)
		}
	}
	for _, id := range GetActAsMains(userID) {
		if !seen[id] {
			seen[id] = true
			res = append(res, id)
		}
	}
	return res
}

// IsActAsLinked reports whether altUserID is currently linked to mainUserID.
func IsActAsLinked(mainUserID, altUserID string) bool {
	FlushPendingSaves()
	count, err := queries.IsActAsLinked(ctx, IsActAsLinkedParams{
		MainUserID: mainUserID,
		AltUserID:  altUserID,
	})
	if err != nil {
		return false
	}
	return count > 0
}

// IsActAsLinkedEither reports whether u1 and u2 are linked in either direction.
func IsActAsLinkedEither(u1, u2 string) bool {
	FlushPendingSaves()
	count, err := queries.IsActAsLinkedEither(ctx, IsActAsLinkedEitherParams{
		MainUserID:   u1,
		AltUserID:    u2,
		MainUserID_2: u2,
		AltUserID_2:  u1,
	})
	if err != nil {
		return false
	}
	return count > 0
}

var (
	ephemeralSwitchesMutex   sync.RWMutex
	ephemeralMessageSwitches = make(map[string]ephemeralSwitchInfo)
)

type ephemeralSwitchInfo struct {
	MainUserID string
	AltUserID  string
	CreatedAt  time.Time
}

// RegisterEphemeralMessageSwitch records that an ephemeral response message was created while acting as altUserID.
func RegisterEphemeralMessageSwitch(messageID, mainUserID, altUserID string) {
	if messageID == "" || altUserID == "" {
		return
	}
	ephemeralSwitchesMutex.Lock()
	defer ephemeralSwitchesMutex.Unlock()

	now := time.Now()
	for k, v := range ephemeralMessageSwitches {
		if now.Sub(v.CreatedAt) > 2*time.Hour {
			delete(ephemeralMessageSwitches, k)
		}
	}
	ephemeralMessageSwitches[messageID] = ephemeralSwitchInfo{
		MainUserID: mainUserID,
		AltUserID:  altUserID,
		CreatedAt:  now,
	}
}

// GetEphemeralMessageSwitch returns the altUserID if the message was launched under an act-as switch.
func GetEphemeralMessageSwitch(messageID string) (string, bool) {
	if messageID == "" {
		return "", false
	}
	ephemeralSwitchesMutex.RLock()
	defer ephemeralSwitchesMutex.RUnlock()
	info, ok := ephemeralMessageSwitches[messageID]
	if !ok {
		return "", false
	}
	return info.AltUserID, true
}

// ClearEphemeralMessageSwitch removes the ephemeral message switch mapping.
func ClearEphemeralMessageSwitch(messageID string) {
	ephemeralSwitchesMutex.Lock()
	defer ephemeralSwitchesMutex.Unlock()
	delete(ephemeralMessageSwitches, messageID)
}

// ActAsForeverExpiry represents an indefinite/forever expiration time for an act-as switch.
var ActAsForeverExpiry = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// IsActAsForever reports whether exp indicates a switch that does not expire.
func IsActAsForever(exp time.Time) bool {
	return exp.IsZero() || exp.Equal(ActAsForeverExpiry) || exp.Year() >= 9999
}

// SetActAsSwitch sets an active act-as switch for mainUserID in channelID until expiresAt.
func SetActAsSwitch(mainUserID, channelID, altUserID string, expiresAt time.Time) error {
	FlushPendingSaves()
	return queries.UpsertActAsSwitch(ctx, UpsertActAsSwitchParams{
		MainUserID: mainUserID,
		ChannelID:  channelID,
		AltUserID:  altUserID,
		ExpiresAt:  expiresAt,
	})
}

// ClearActAsSwitch removes the active act-as switch for mainUserID in channelID.
func ClearActAsSwitch(mainUserID, channelID string) error {
	FlushPendingSaves()
	_, err := queries.DeleteActAsSwitch(ctx, DeleteActAsSwitchParams{
		MainUserID: mainUserID,
		ChannelID:  channelID,
	})
	return err
}

// ClearAllActAsSwitches removes all active act-as switches for mainUserID across all channels.
func ClearAllActAsSwitches(mainUserID string) error {
	FlushPendingSaves()
	_, err := queries.DeleteAllActAsSwitchesForUser(ctx, DeleteAllActAsSwitchesForUserParams{
		MainUserID: mainUserID,
		AltUserID:  mainUserID,
	})
	return err
}

// MoveActAsSwitch transfers an active act-as switch for mainUserID from fromChannelID to toChannelID, preserving expiry.
func MoveActAsSwitch(mainUserID, fromChannelID, toChannelID string) error {
	altID, exp, active := GetActAsSwitch(mainUserID, fromChannelID)
	if !active || altID == "" {
		return nil
	}
	return MoveActAsSwitchWithExpiry(mainUserID, fromChannelID, toChannelID, exp)
}

// MoveActAsSwitchWithExpiry transfers an active act-as switch for mainUserID from fromChannelID to toChannelID with a new expiry.
func MoveActAsSwitchWithExpiry(mainUserID, fromChannelID, toChannelID string, newExpiry time.Time) error {
	if fromChannelID == toChannelID || fromChannelID == "" || toChannelID == "" {
		return nil
	}
	altID, _, active := GetActAsSwitch(mainUserID, fromChannelID)
	if !active || altID == "" {
		return nil
	}
	err := SetActAsSwitch(mainUserID, toChannelID, altID, newExpiry)
	if err == nil {
		_ = ClearActAsSwitch(mainUserID, fromChannelID)
	}
	return err
}

// GetActAsSwitch returns the active switch if unexpired and valid for channelID.
func GetActAsSwitch(mainUserID, channelID string) (altUserID string, expiresAt time.Time, active bool) {
	if channelID == "" {
		return "", time.Time{}, false
	}
	FlushPendingSaves()
	sw, err := queries.GetActAsSwitch(ctx, GetActAsSwitchParams{
		MainUserID: mainUserID,
		ChannelID:  channelID,
	})
	if err != nil {
		return "", time.Time{}, false
	}
	if !IsActAsForever(sw.ExpiresAt) && time.Now().After(sw.ExpiresAt) {
		_, _ = queries.DeleteActAsSwitch(ctx, DeleteActAsSwitchParams{
			MainUserID: mainUserID,
			ChannelID:  channelID,
		})
		return "", time.Time{}, false
	}
	if !IsActAsLinked(mainUserID, sw.AltUserID) {
		_, _ = queries.DeleteActAsSwitch(ctx, DeleteActAsSwitchParams{
			MainUserID: mainUserID,
			ChannelID:  channelID,
		})
		return "", time.Time{}, false
	}
	return sw.AltUserID, sw.ExpiresAt, true
}

// GetEffectiveUserID returns altUserID if mainUserID has an active, unexpired switch in any of contextIDs; otherwise mainUserID.
// Checks ephemeral message switch registrations first, then channel/thread switches.
func GetEffectiveUserID(userID string, contextIDs ...string) string {
	eff, _, _ := GetEffectiveUserIDAndExpiry(userID, contextIDs...)
	return eff
}

// GetEffectiveUserIDAndExpiry returns the effective user ID, switch expiry, and whether an active switch is in place.
// Checks ephemeral message switch registrations first, then channel/thread switches.
func GetEffectiveUserIDAndExpiry(userID string, contextIDs ...string) (effectiveID string, expiresAt time.Time, isActingAs bool) {
	// First check if any contextID corresponds to a registered ephemeral response message switch
	for _, id := range contextIDs {
		if id == "" {
			continue
		}
		if altID, ok := GetEphemeralMessageSwitch(id); ok {
			return altID, time.Time{}, true
		}
	}

	// Next check active switches for each channel/thread contextID
	for _, id := range contextIDs {
		if id == "" {
			continue
		}
		if altID, exp, active := GetActAsSwitch(userID, id); active && altID != "" {
			return altID, exp, true
		}
	}

	return userID, time.Time{}, false
}
