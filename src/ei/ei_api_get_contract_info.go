package ei

import (
	"log"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"google.golang.org/protobuf/proto"
)

// GetContractsInfoFromAPI fetches information for a set of contract identifiers
// from the Egg Inc API endpoint ei_ctx/get_contracts_info.
//
// Parameters:
//   - contractIdentifiers ([]string): the list of contract IDs to query.
//
// Returns:
//   - (*ContractsInfoResponse): the decoded contracts info response, or nil on error.
func GetContractsInfoFromAPI(contractIdentifiers []string) *ContractsInfoResponse {
	reqURL := "https://www.auxbrain.com/ei_ctx/get_contracts_info"

	clientVersion := DefaultClientVersion
	version := DefaultVersion
	build := DefaultBuild
	platformString := DefaultPlatformString

	request := ContractsInfoRequest{
		Rinfo: &BasicRequestInfo{
			EiUserId:      &config.EIUserID,
			ClientVersion: &clientVersion,
			Version:       &version,
			Build:         &build,
			Platform:      &platformString,
		},
		ContractIdentifiers: contractIdentifiers,
		ClientVersion:       proto.Uint32(clientVersion),
	}

	payload, _ := APICall(reqURL, &request, false, 0, "", true)
	if payload == nil {
		log.Print("GetContractsInfoFromAPI: APICall returned nil response")
		return nil
	}

	response := &ContractsInfoResponse{}
	opts := proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
	if err := opts.Unmarshal(payload, response); err != nil {
		log.Print(err)
		return nil
	}

	return response
}
