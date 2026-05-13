package ei

import (
	"log"

	"google.golang.org/protobuf/proto"
)

// GetLeaderboardFromAPI fetches the leaderboard for a given scope and player grade
// from the Egg Inc API endpoint ei_ctx/get_leaderboard.
//
// Parameters:
//   - eggIncID (string): the player's Egg Inc user ID (will be decrypted if needed).
//   - scope (string): the leaderboard scope (season ID or all-time scope string).
//   - grade (Contract_PlayerGrade): the player grade to query.
//
// Returns:
//   - (*LeaderboardResponse): the decoded leaderboard response, or nil on error.
func GetLeaderboardFromAPI(eggIncID string, scope string, grade Contract_PlayerGrade) *LeaderboardResponse {
	eiUserID := DecryptEID(eggIncID)
	reqURL := "https://www.auxbrain.com//ei_ctx/get_leaderboard"

	clientVersion := DefaultClientVersion
	version := DefaultVersion
	build := DefaultBuild
	platformString := DefaultPlatformString

	leaderboardRequest := LeaderboardRequest{
		Rinfo: &BasicRequestInfo{
			EiUserId:      &eiUserID,
			ClientVersion: &clientVersion,
			Version:       &version,
			Build:         &build,
			Platform:      &platformString,
		},
		Scope: &scope,
		Grade: &grade,
	}

	payload := APIAuthenticatedCall(reqURL, &leaderboardRequest)
	if payload == nil {
		log.Print("GetLeaderboardFromAPI: APICall returned nil response")
		return nil
	}

	leaderboardResponse := &LeaderboardResponse{}
	opts := proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
	if err := opts.Unmarshal(payload, leaderboardResponse); err != nil {
		log.Print(err)
		return nil
	}

	return leaderboardResponse
}

// GetLeaderboardInfoFromAPI fetches leaderboard season metadata and all-time scope
// from the Egg Inc API endpoint ei_ctx/get_leaderboard_info.
//
// Parameters:
//   - eggIncID (string): the player's Egg Inc user ID (will be decrypted if needed).
//
// Returns:
//   - (*LeaderboardInfo): the decoded leaderboard info response, or nil on error.
func GetLeaderboardInfoFromAPI(eggIncID string) *LeaderboardInfo {
	eiUserID := DecryptEID(eggIncID)
	reqURL := "https://www.auxbrain.com//ei_ctx/get_leaderboard_info"

	clientVersion := DefaultClientVersion
	version := DefaultVersion
	build := DefaultBuild
	platformString := DefaultPlatformString

	request := BasicRequestInfo{
		EiUserId:      &eiUserID,
		ClientVersion: &clientVersion,
		Version:       &version,
		Build:         &build,
		Platform:      &platformString,
	}

	payload := APICall(reqURL, &request)
	if payload == nil {
		log.Print("GetLeaderboardInfoFromAPI: APICall returned nil response")
		return nil
	}

	info := &LeaderboardInfo{}
	opts := proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
	if err := opts.Unmarshal(payload, info); err != nil {
		log.Print(err)
		return nil
	}

	return info
}
