package boost

import (
	"testing"
)

func TestGetVersionAndRevisionInfo(t *testing.T) {
	runningVer, runningRev, _, diskVer, diskRev, _, _ := getVersionAndRevisionInfo()

	if runningVer == "" {
		t.Errorf("expected runningVer to not be empty")
	}
	t.Logf("runningVer=%q, runningRev=%q, diskVer=%q, diskRev=%q", runningVer, runningRev, diskVer, diskRev)
}

func TestLdflagsVersionRegex(t *testing.T) {
	tests := []struct {
		ldflags string
		want    string
	}{
		{`-s -w -X main.Version=v7.0-203-ge42cb8920-dirty`, `v7.0-203-ge42cb8920-dirty`},
		{`-X main.Version=v7.0-100`, `v7.0-100`},
		{`-X 'main.Version=v7.0-99'`, `v7.0-99`},
		{`-X "main.Version=v7.0-98"`, `v7.0-98`},
		{`-X Version=v1.2.3`, `v1.2.3`},
	}

	for _, tt := range tests {
		m := ldflagsVersionRegex.FindStringSubmatch(tt.ldflags)
		if len(m) < 2 || m[1] != tt.want {
			t.Errorf("ldflagsVersionRegex(%q) = %v; want %q", tt.ldflags, m, tt.want)
		}
	}
}
