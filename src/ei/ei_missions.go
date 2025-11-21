package ei

import (
	"encoding/json"
)

const missionJSON = `{"ships":[
	{"name": "Chicken One","art":"chicken1","duration":["20m","1h","2h"]},
	{"name": "Chicken Nine","art":"chicken9","duration":["30m","1h","3h"]},
	{"name": "Chicken Heavy","art":"chickenheavy","duration":["45m","1h30m","4h"]},
	{"name": "BCR","art":"bcr","duration":["1h30m","4h","8h"]},
	{"name": "Quintillion Chicken","art":"milleniumchicken","duration":["3h","6h","12h"]},
	{"name": "Cornish-Hen Corvette","art":"corellihencorvette","duration":["4h","12h","1d"]},
	{"name": "Galeggtica","art":"galeggtica","duration":["6h","16h","1d6h"]},
	{"name": "Defihent","art":"defihent","duration":["8h","1d","2d"]},
	{"name": "Voyegger","art":"voyegger","duration":["12h","1d12h","3d"]},
	{"name": "Henerprise","art":"henerprise","duration":["1d","2d","4d"]},
	{"name": "Atreggies Henliner","art":"atreggies","duration":["2d","3d","4d"]}
	]}`

type missionData struct {
	Ships []struct {
		Name     string   `json:"name"`
		Art      string   `json:"art"`
		Duration []string `json:"duration"`
	} `json:"ships"`
}

// MissionArt holds the mission art and durations loaded from JSON
var MissionArt missionData

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &MissionArt)
}
