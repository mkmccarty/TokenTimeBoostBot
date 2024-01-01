package farmerstate

import (
	"strconv"
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

func TestSetOrderPercentileOne(t *testing.T) {
	SetOrderPercentileOne("TEST", 1, 10)

	index := len(farmerstate["TEST"].OrderHistory) - 1
	if farmerstate["TEST"].OrderHistory[index] != 10 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TEST"].OrderHistory[index], 10)
	}
	SetOrderPercentileOne("TEST", 9, 10)
	index = len(farmerstate["TEST"].OrderHistory) - 1
	if farmerstate["TEST"].OrderHistory[index] != 90 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TEST"].OrderHistory[index], 90)
	}

}

func TestSetOrderPercentileAll(t *testing.T) {
	order := []string{"TEST1", "TEST2", "TEST3", "TEST4", "TEST5", "TEST6", "TEST7", "TEST8", "TEST9", "TEST10"}

	SetOrderPercentileAll(order, len(order))

	// create loop from 1 to 10
	for i := 1; i <= len(order); i++ {
		// test against last entry from OrderHistory
		if farmerstate["TEST"+strconv.Itoa(i)].OrderHistory[len(farmerstate["TEST"+strconv.Itoa(i)].OrderHistory)-1] != i*10 {
			t.Errorf("SetOrderPercentileAll() = %v, want %v", farmerstate["TEST"+strconv.Itoa(i)].OrderHistory[len(farmerstate["TEST"+strconv.Itoa(i)].OrderHistory)-1], i*10)
		}
	}

}
