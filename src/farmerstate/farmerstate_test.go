package farmerstate

import (
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
