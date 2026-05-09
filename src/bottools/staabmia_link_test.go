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
		"T4R Defl.",
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
