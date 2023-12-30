package farmerstate

import (
	"testing"
)

func TestSetTokens(t *testing.T) {
	SetTokens("TEST", 5)

	if GetTokens("TEST") != farmerstate["TEST"].tokens {
		t.Errorf("SetTokens() = %v, want %v", GetTokens("TEST"), farmerstate["TEST"].tokens)
	}
}

func TestGetPing(t *testing.T) {
	SetPing("TEST", true)

	if GetPing("TEST") != farmerstate["TEST"].ping {
		t.Errorf("SetTokens() = %v, want %v", GetPing("TEST"), farmerstate["TEST"].ping)
	}

}
