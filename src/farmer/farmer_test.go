package farmer

import (
	"testing"
)

func TestSetTokens(t *testing.T) {
	SetTokens("TEST", 5)

	if GetTokens("TEST") != farmers["TEST"].tokens {
		t.Errorf("SetTokens() = %v, want %v", GetTokens("TEST"), farmers["TEST"].tokens)
	}
}

func TestGetPing(t *testing.T) {
	SetPing("TEST", true)

	if GetPing("TEST") != farmers["TEST"].ping {
		t.Errorf("SetTokens() = %v, want %v", GetPing("TEST"), farmers["TEST"].ping)
	}

}
