package ei

import "encoding/json"

const missionJSON = `{"ships":[
	{"name": "Chicken One","art":"<:chicken1:1280045945974951949>","artDev":"<:chicken1:1280390988824576061>","duration":["20m","1h","2h"]},
	{"name": "Chicken Nine","art":"<:chicken9:1280045842442616902>","artDev":"<:chicken9:1280390884575154226>","duration":["30m","1h","3h"]},
	{"name": "Chicken Heavy","art":"<:chickenheavy:1280045643922018315>","artDev":"<:chickenheavy:1280390782783590473>","duration":["45m","1h30m","4h"]},
	{"name": "BCR","art":"<:bcr:1280045542495228008>","artDev":"<:bcr:1280390686461661275>","duration":["1h30m","4h","8h"]},
	{"name": "Quintillion Chicken","art":"<:milleniumchicken:1280045411444326400>","artDev":"<:milleniumchicken:1280390575178383386>","duration":["3h","6h","12h"]},
	{"name": "Cornish-Hen Corvette","art":"<:corellihencorvette:1280045137518657536>","artDev":"<:corellihencorvette:1280390458983452742>","duration":["4h","12h","1d"]},
	{"name": "Galeggtica","art":"<:galeggtica:1280045010917527593>","artDev":"<:galeggtica:1280390347825872916>","duration":["6h","16h","1d6h"]},
	{"name": "Defihent","art":"<:defihent:1280044758001258577>","artDev":"<:defihent:1280390249943666739>","duration":["8h","1d","2d"]},
	{"name": "Voyegger","art":"<:voyegger:1280041822416273420>","artDev":"<:voyegger:1280390114094354472>","duration":["12h","1d12h","3d"]},
	{"name": "Henerprise","art":"<:henerprise:1280038539328749609>","artDev":"<:henerprise:1280390026487664704>","duration":["1d","2d","4d"]},
	{"name": "Atreggies Henliner","art":"<:atreggies:1280038674398183464>","artDev":"<:atreggies:1280389911509340240>","duration":["2d","3d","4d"]}
	]}`

type missionData struct {
	Ships []struct {
		Name     string   `json:"name"`
		Art      string   `json:"art"`
		ArtDev   string   `json:"artDev"`
		Duration []string `json:"duration"`
	} `json:"ships"`
}

// MissionArt holds the mission art and durations loaded from JSON
var MissionArt missionData

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &MissionArt)
}
