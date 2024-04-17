package launch

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var integerZeroMinValue float64 = 0.0

type shipData struct {
	Name     string   `json:"Name"`
	Art      string   `json:"Art"`
	Duration []string `json:"Duration"`
}

type missionData struct {
	Ships []shipData
}

const missionJSON = `{"ships":[
	{"name": "Atreggies Henliner","art":"","duration":["2d","3d","4d"]},
	{"name": "Henerprise","art":"","duration":["1d","2d","4d"]},
	{"name": "Voyegger","art":"","duration":["12h","1d12h","3d"]},
	{"name": "Defihent","art":"","duration":["8h","1d","2d"]},
	{"name": "Galeggtica","art":"","duration":["6h","16h","1d6h"]},
	{"name": "Cornish-Hen Corvette","art":"","duration":["4h","12h","1d"]},
	{"name": "Quintillion Chicken","art":"","duration":["3h","6h","12h"]},
	{"name": "BCR","art":"","duration":["1h30m","4h","8h"]},
	{"name": "Chicken Heavy","art":"","duration":["45m","1h30m","4h"]},
	{"name": "Chicken Nine","art":"","duration":["30m","1h","3h"]},
	{"name": "Chicken One","art":"","duration":["20m","1h","2h"]}
	]}`

var mis missionData

func init() {
	json.Unmarshal([]byte(missionJSON), &mis)
}

func fmtDuration(d time.Duration) string {
	str := ""
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d = h / 24
	h -= d * 24

	if d > 0 {
		str = fmt.Sprintf("%dd%dh%dm", d, h, m)
	} else {
		str = fmt.Sprintf("%dh%dm", h, m)
	}
	return strings.Replace(str, "0h0m", "", -1)
}
