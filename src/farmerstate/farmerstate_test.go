package farmerstate

import (
	"testing"
)

func TestSetTokens(t *testing.T) {
	SetTokens("TEST", 5)

	if GetTokens("TEST") != farmerstate["TEST"].Tokens {
		t.Errorf("SetTokens() = %v, want %v", GetTokens("TEST"), farmerstate["TEST"].Tokens)
	}
}

func TestGetPing(t *testing.T) {
	SetPing("TEST", true)

	if GetPing("TEST") != farmerstate["TEST"].Ping {
		t.Errorf("SetTokens() = %v, want %v", GetPing("TEST"), farmerstate["TEST"].Ping)
	}

}

func TestSetOrderPercentile(t *testing.T) {
	SetOrderPercentile("TEST", 1, 10)

	index := len(farmerstate["TEST"].OrderHistory) - 1
	if farmerstate["TEST"].OrderHistory[index] != 10 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TEST"].OrderHistory[index], 10)
	}
	SetOrderPercentile("TEST", 9, 10)
	index = len(farmerstate["TEST"].OrderHistory) - 1
	if farmerstate["TEST"].OrderHistory[index] != 90 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TEST"].OrderHistory[index], 90)
	}

}
