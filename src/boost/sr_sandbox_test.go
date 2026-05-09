package boost

import (
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

func TestFmtNumberSingleUnit(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		sandbox  bool
		expected string
		unit     int
	}{
		{"1.3Q sandbox", 1.3e18, true, "1p3", 1},
		{"500q sandbox", 500e15, true, "500", 0},
		{"500T sandbox", 500e12, true, "500", 2},
		{"2.5Q standard", 2.5e18, false, "2.5", 0},
		{"2.5q standard", 2.5e15, false, "2.5", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, gotUnit := FmtNumberSingleUnit(tt.value, tt.sandbox)
			if gotStr != tt.expected || gotUnit != tt.unit {
				t.Errorf("FmtNumberSingleUnit() = (%v, %v), want (%v, %v)", gotStr, gotUnit, tt.expected, tt.unit)
			}
		})
	}
}

func base62Decode(s string) uint64 {
	var result uint64
	for _, c := range s {
		result = result*62 + uint64(strings.IndexRune(string(base62Charset), c))
	}
	return result
}

func unchunk16(dataEncoded62 string) string {
	x := ""
	inputLength := 9
	if len(dataEncoded62) < 10 {
		return strconv.FormatUint(base62Decode(dataEncoded62), 10)
	}

	n := len(dataEncoded62) / inputLength
	for i := 0; i < n; i++ {
		chunk := dataEncoded62[i*inputLength : (i+1)*inputLength]
		valStr := strconv.FormatUint(base62Decode(chunk), 10)
		if len(valStr) > 0 {
			x += valStr[1:]
		}
	}
	tmp := dataEncoded62[n*inputLength:]
	if len(tmp) > 0 {
		valStr := strconv.FormatUint(base62Decode(tmp), 10)
		if len(valStr) > 0 {
			x += valStr[1:]
		}
	}
	return x
}

func TestDecodeURLData(t *testing.T) {
	urlData := "v_5UGxheWVyJTdDMC0xMTAwMDAxLTAtMTMtNy0zODAtNjAtMXAwLTItMXA4NzUtMi02LTUw=B6mEavjeExzag&c=QmFja2VkJTIwVXA"

	// 1. Separate '&c=' part if present
	parts := strings.Split(urlData, "&c=")
	payload := parts[0]
	var contractName string
	if len(parts) > 1 {
		decodedC, err := base64.RawStdEncoding.DecodeString(parts[1])
		if err == nil {
			contractName, _ = url.QueryUnescape(string(decodedC))
		}
	}

	// 2. Remove v_5 prefix
	payload = strings.TrimPrefix(payload, "v_5")

	// 3. Split by '=' to get base64 and base62 parts
	dataParts := strings.Split(payload, "=")
	if len(dataParts) != 2 {
		t.Fatalf("Expected 2 parts separated by '=', got %d", len(dataParts))
	}

	// 4. Decode base64
	base64Decoded, err := base64.StdEncoding.DecodeString(dataParts[0])
	if err != nil {
		t.Fatalf("Base64 decode failed: %v", err)
	}

	unescapedData1, err := url.QueryUnescape(string(base64Decoded))
	if err != nil {
		t.Fatalf("QueryUnescape failed: %v", err)
	}

	// 5. Decode base62 (artifacts matrix)
	data2 := unchunk16(dataParts[1])

	// 6. Validate expected values match the encoding implementation
	expectedContractName := "Backed Up"
	if contractName != expectedContractName {
		t.Errorf("Expected contract name to be %q, got %q", expectedContractName, contractName)
	}

	expectedData1 := "Player|0-1100001-0-13-7-380-60-1p0-2-1p875-2-6-50"
	if unescapedData1 != expectedData1 {
		t.Errorf("Expected data1 to be %q, got %q", expectedData1, unescapedData1)
	}

	expectedData2 := "11000000141414000000"
	if data2 != expectedData2 {
		t.Errorf("Expected data2 to be %q, got %q", expectedData2, data2)
	}
}

func TestBase62Encode(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0"},
		{61, "Z"},
		{62, "10"},
		{12345, "3d7"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := base62Encode(tt.input); got != tt.expected {
				t.Errorf("base62Encode(%d) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestChunk16(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"small string", "12345"},
		{"exact 15 chars", "123456789012345"},
		{"exact 16 chars", "1234567890123456"},
		{"large string", "1100000013750060112"},
		{"all ones", "11111111111111111111111111111111"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := chunk16(tt.input)
			if encoded == "" {
				t.Errorf("chunk16 returned empty string for input %v", tt.input)
			}
		})
	}
}

func TestGatherData(t *testing.T) {
	input := inputData{
		crtToggle:   true,
		tokenToggle: true,
		ggToggle:    false,
		eggUnit:     1,
		durUnit:     0,
		modName:     2,
		cxpToggle:   true,
		crtTime:     "0",
		mpft:        "13",
		duration:    "8",
		targetEgg:   "1p3",
		tokenTimer:  "60",
		modifiers:   "0p5",
		numPlayers:  8,
		btvTarget:   "1p875",
		players: []SandboxPlayer{
			{
				Name:         "TestPlayer",
				Tokens:       "5",
				TE:           "50",
				Mirror:       false,
				Colleggtible: false,
				Sink:         false,
				Creator:      false,
				Item1:        "00", Item2: "00", Item3: "00", Item4: "00",
				Item5: "00", Item6: "00", Item7: "00", Item8: "00",
			},
		},
	}

	d1, d2, err := gatherData(input, "Delugge", "delugge-2023")
	if err != nil {
		t.Fatalf("gatherData failed: %v", err)
	}

	expectedD1Start := "Delugge|delugge-2023-1101021-0-13-8-1p3-60-0p5-8-1p875-TestPlayer-5-50"
	if !strings.HasPrefix(d1, expectedD1Start) {
		t.Errorf("Expected D1 to start with %q, got %q", expectedD1Start, d1)
	}

	if d2 == "" {
		t.Errorf("Expected D2 to be populated")
	}
}

func TestEncodeSandboxData(t *testing.T) {
	c := &ei.EggIncContract{
		ID:         "delugge-2023",
		Name:       "Delugge",
		ModifierSR: 0.5,
	}
	players := []SandboxPlayer{
		{
			Name:         "Player 1",
			Tokens:       "5",
			TE:           "50",
			Mirror:       false,
			Colleggtible: true,
			Sink:         false,
			Creator:      false,
			Item1:        "00", Item2: "00", Item3: "00", Item4: "00",
			Item5: "00", Item6: "00", Item7: "00", Item8: "00",
		},
	}

	encoded, err := EncodeSandboxData(true, 1.3e18, "60", 8*86400, 8, c, players)
	if err != nil {
		t.Fatalf("EncodeSandboxData failed: %v", err)
	}

	if !strings.HasPrefix(encoded, "data=v_5") {
		t.Errorf("Expected data=v_5 prefix, got %v", encoded)
	}

	if !strings.Contains(encoded, "=") {
		t.Errorf("Expected base64 and base62 parts separated by =, got %v", encoded)
	}

	if !strings.Contains(encoded, "&c=") {
		t.Errorf("Expected contract parameter &c=, got %v", encoded)
	}
}
