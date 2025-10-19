package farmerstate

import (
	"strconv"
	"testing"
)

func TestSetEggIncName(t *testing.T) {
	SetEggIncName("TestUser", "TestEggIncName")

	if GetEggIncName("TestUser") != farmerstate["TestUser"].EggIncName {
		t.Errorf("SetEggIncName() = %v, want %v", GetEggIncName("TestUser"), farmerstate["TestUser"].EggIncName)
	}
}

func TestGetEggIncName(t *testing.T) {
	SetEggIncName("TestUser", "TestEggIncName")

	if GetEggIncName("TestUser") != farmerstate["TestUser"].EggIncName {
		t.Errorf("GetEggIncName() = %v, want %v", GetEggIncName("TestUser"), farmerstate["TestUser"].EggIncName)
	}
}

func TestSetTokens(t *testing.T) {
	SetTokens("TestUser", 5)

	if GetTokens("TestUser") != farmerstate["TestUser"].Tokens {
		t.Errorf("SetTokens() = %v, want %v", GetTokens("TestUser"), farmerstate["TestUser"].Tokens)
	}
}

func TestGetPing(t *testing.T) {
	SetPing("TestUser", true)

	if GetPing("TestUser") != farmerstate["TestUser"].Ping {
		t.Errorf("SetTokens() = %v, want %v", GetPing("TestUser"), farmerstate["TestUser"].Ping)
	}

}

func TestSetOrderPercentileOne(t *testing.T) {
	SetOrderPercentileOne("TestUser", 1, 10)

	index := len(farmerstate["TestUser"].OrderHistory) - 1
	if farmerstate["TestUser"].OrderHistory[index] != 10 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TestUser"].OrderHistory[index], 10)
	}
	SetOrderPercentileOne("TestUser", 9, 10)
	index = len(farmerstate["TestUser"].OrderHistory) - 1
	if farmerstate["TestUser"].OrderHistory[index] != 90 {
		t.Errorf("SetOrderPercentile() = %v, want %v", farmerstate["TestUser"].OrderHistory[index], 90)
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

func TestGetOrderHistory(t *testing.T) {

	order := []string{"TestUser", "TEST_MISSING"}

	DeleteFarmer("TEST_MISSING")
	GetOrderHistory(order, 5)

	if farmerstate["TEST_MISSING"].OrderHistory[0] != 50 {
		t.Errorf("GetOrderHistory() = %v, want %v", farmerstate["TEST_MISSING"].OrderHistory[0], 50)
	}
}
