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

func TestSetOrderPercentile(t *testing.T) {
	SetOrderPercentile("TEST", 1, 10)

	index := len(farmerstate["TEST"].order_history) - 1
	if farmerstate["TEST"].order_history[index] != 10 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TEST"].order_history[index], 10)
	}
	SetOrderPercentile("TEST", 9, 10)
	index = len(farmerstate["TEST"].order_history) - 1
	if farmerstate["TEST"].order_history[index] != 90 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TEST"].order_history[index], 90)
	}

}
