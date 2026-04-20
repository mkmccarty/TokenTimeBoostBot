package boost

import (
	"reflect"
	"testing"
)

func TestParseFarmerInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []ParsedFarmer
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "single guest",
			input: "Bob",
			want:  []ParsedFarmer{{Guest: "Bob"}},
		},
		{
			name:  "single mention",
			input: "<@123456789>",
			want:  []ParsedFarmer{{Mention: "<@123456789>"}},
		},
		{
			name:  "mixed multiple",
			input: "Alice, <@987654321>, Charlie",
			want: []ParsedFarmer{
				{Guest: "Alice"},
				{Mention: "<@987654321>"},
				{Guest: "Charlie"},
			},
		},
		{
			name:  "with extra spaces and empty elements",
			input: "  Dave  ,, <@111222333> , ",
			want: []ParsedFarmer{
				{Guest: "Dave"},
				{Mention: "<@111222333>"},
			},
		},
		{
			name:  "multiple mentions no spaces",
			input: "<@987654321><@123456789><@111222333>,<@444555666>",
			want: []ParsedFarmer{
				{Mention: "<@987654321>"},
				{Mention: "<@123456789>"},
				{Mention: "<@111222333>"},
				{Mention: "<@444555666>"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseFarmerInput(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseFarmerInput() = %v, want %v", got, tt.want)
			}
		})
	}
}
