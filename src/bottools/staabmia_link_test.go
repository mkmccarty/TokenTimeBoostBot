package bottools

import "testing"

func TestGetSandboxItemIndices(t *testing.T) {
	metro, comp, gusset, defl := GetSandboxItemIndices([]string{
		"T4L Metro",
		"T4E Comp",
		"T4C Gusset",
		"T4R Defl.",
	})

	if metro != "00" {
		t.Fatalf("metro = %q, want %q", metro, "00")
	}
	if comp != "01" {
		t.Fatalf("comp = %q, want %q", comp, "01")
	}
	if gusset != "04" {
		t.Fatalf("gusset = %q, want %q", gusset, "04")
	}
	if defl != "02" {
		t.Fatalf("defl = %q, want %q", defl, "02")
	}
}

func TestGetSandboxIHRItemIndices(t *testing.T) {
	chalice, monocle, defl, siab := GetSandboxIHRItemIndices([]string{
		"T4L Chalice",
		"T4E Monocle",
		"T4R IHR Defl.",
		"T4C SIAB",
	})

	if chalice != "00" {
		t.Fatalf("chalice = %q, want %q", chalice, "00")
	}
	if monocle != "01" {
		t.Fatalf("monocle = %q, want %q", monocle, "01")
	}
	if defl != "02" {
		t.Fatalf("defl = %q, want %q", defl, "02")
	}
	if siab != "03" {
		t.Fatalf("siab = %q, want %q", siab, "03")
	}
}

func TestSandboxDeflectorsStaySeparate(t *testing.T) {
	artifacts := []string{
		"T4L Defl.",
		"T4L IHR Defl.",
	}

	_, _, _, defl := GetSandboxItemIndices(artifacts)
	_, _, ihrDefl, _ := GetSandboxIHRItemIndices(artifacts)

	if defl != "00" {
		t.Fatalf("defl = %q, want %q", defl, "00")
	}
	if ihrDefl != "00" {
		t.Fatalf("ihrDefl = %q, want %q", ihrDefl, "00")
	}
}

func TestNormalDeflectorDoesNotFillIHRSlot(t *testing.T) {
	chalice, monocle, ihrDefl, siab := GetSandboxIHRItemIndices([]string{"T4L Defl."})

	if chalice != "09" {
		t.Fatalf("chalice = %q, want %q (Empty)", chalice, "09")
	}
	if monocle != "06" {
		t.Fatalf("monocle = %q, want %q (Empty)", monocle, "06")
	}
	if ihrDefl != "10" {
		t.Fatalf("ihrDefl = %q, want %q (Empty)", ihrDefl, "10")
	}
	if siab != "08" {
		t.Fatalf("siab = %q, want %q (Empty)", siab, "08")
	}
}

func TestGetSandboxItemIndices_PrefersHighestRankPerSlot(t *testing.T) {
	metro, comp, gusset, defl := GetSandboxItemIndices([]string{
		"T4L Defl.",
		"T4L Metro",
		"T4L Comp",
		"T4L Gusset",
		"T3R Chalice",
		"T4L Monocle",
		"T4L IHR Defl.",
		"T4E SIAB",
	})

	if metro != "00" {
		t.Fatalf("metro = %q, want %q", metro, "00")
	}
	if comp != "00" {
		t.Fatalf("comp = %q, want %q", comp, "00")
	}
	if gusset != "00" {
		t.Fatalf("gusset = %q, want %q", gusset, "00")
	}
	if defl != "00" {
		t.Fatalf("defl = %q, want %q", defl, "00")
	}
}

func TestGetSandboxIHRItemIndices_PrefersHighestRankPerSlot(t *testing.T) {
	chalice, monocle, ihrDefl, siab := GetSandboxIHRItemIndices([]string{
		"T3R Chalice",
		"T4L Chalice",
		"T2C Monocle",
		"T4L Monocle",
		"T4E IHR Defl.",
		"T4L IHR Defl.",
		"T4E SIAB",
		"T4L SIAB",
	})

	if chalice != "00" {
		t.Fatalf("chalice = %q, want %q", chalice, "00")
	}
	if monocle != "00" {
		t.Fatalf("monocle = %q, want %q", monocle, "00")
	}
	if ihrDefl != "00" {
		t.Fatalf("ihrDefl = %q, want %q", ihrDefl, "00")
	}
	if siab != "00" {
		t.Fatalf("siab = %q, want %q", siab, "00")
	}
}
