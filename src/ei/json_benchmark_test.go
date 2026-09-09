package ei

import (
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func findDataPath(rel string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	// dir is src/ei, workspace root is ../..
	root := filepath.Clean(filepath.Join(dir, "../.."))
	return filepath.Join(root, rel)
}

func loadFileData(b *testing.B, rel string) []byte {
	path := findDataPath(rel)
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

// -------------------------------------------------------------
// 1. ttbb-data/ei-contracts.json (~1.2MB) -> []EggIncContract
// -------------------------------------------------------------

func BenchmarkContracts_Unmarshal_V1(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-contracts.json")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var contracts []EggIncContract
		if err := json.Unmarshal(data, &contracts); err != nil {
			b.Fatalf("v1 unmarshal failed: %v", err)
		}
	}
}

func BenchmarkContracts_Unmarshal_V2(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-contracts.json")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var contracts []EggIncContract
		if err := jsonv2.Unmarshal(data, &contracts); err != nil {
			b.Fatalf("v2 unmarshal failed: %v", err)
		}
	}
}

func BenchmarkContracts_Marshal_V1(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-contracts.json")
	var contracts []EggIncContract
	if err := json.Unmarshal(data, &contracts); err != nil {
		b.Fatalf("setup failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	var totalBytes int64
	for i := 0; i < b.N; i++ {
		out, err := json.Marshal(contracts)
		if err != nil {
			b.Fatalf("v1 marshal failed: %v", err)
		}
		totalBytes = int64(len(out))
	}
	b.SetBytes(totalBytes)
}

func BenchmarkContracts_Marshal_V2(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-contracts.json")
	var contracts []EggIncContract
	if err := json.Unmarshal(data, &contracts); err != nil {
		b.Fatalf("setup failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	var totalBytes int64
	for i := 0; i < b.N; i++ {
		out, err := jsonv2.Marshal(contracts, json.FormatDurationAsNano(true))
		if err != nil {
			b.Fatalf("v2 marshal failed: %v", err)
		}
		totalBytes = int64(len(out))
	}
	b.SetBytes(totalBytes)
}

// -------------------------------------------------------------
// 2. ttbb-data/ei-events.json (~646KB) -> []EggEvent
// -------------------------------------------------------------

func BenchmarkEvents_Unmarshal_V1(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-events.json")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var events []EggEvent
		if err := json.Unmarshal(data, &events); err != nil {
			b.Fatalf("v1 unmarshal failed: %v", err)
		}
	}
}

func BenchmarkEvents_Unmarshal_V2(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-events.json")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var events []EggEvent
		if err := jsonv2.Unmarshal(data, &events); err != nil {
			b.Fatalf("v2 unmarshal failed: %v", err)
		}
	}
}

func BenchmarkEvents_Marshal_V1(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-events.json")
	var events []EggEvent
	if err := json.Unmarshal(data, &events); err != nil {
		b.Fatalf("setup failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	var totalBytes int64
	for i := 0; i < b.N; i++ {
		out, err := json.Marshal(events)
		if err != nil {
			b.Fatalf("v1 marshal failed: %v", err)
		}
		totalBytes = int64(len(out))
	}
	b.SetBytes(totalBytes)
}

func BenchmarkEvents_Marshal_V2(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-events.json")
	var events []EggEvent
	if err := json.Unmarshal(data, &events); err != nil {
		b.Fatalf("setup failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	var totalBytes int64
	for i := 0; i < b.N; i++ {
		out, err := jsonv2.Marshal(events)
		if err != nil {
			b.Fatalf("v2 marshal failed: %v", err)
		}
		totalBytes = int64(len(out))
	}
	b.SetBytes(totalBytes)
}

// -------------------------------------------------------------
// 3. ttbb-data/ei-afx-data.json (~340KB) -> Store (deep hierarchy)
// -------------------------------------------------------------

func BenchmarkArtifactsData_Unmarshal_V1(b *testing.B) {
	raw := loadFileData(b, "ttbb-data/ei-afx-data.json")
	data := []byte(strings.ReplaceAll(string(raw), "./data.schema.json", "./ttbb-data/data.schema.json"))
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var s Store
		if err := json.Unmarshal(data, &s); err != nil {
			b.Fatalf("v1 unmarshal failed: %v", err)
		}
	}
}

func BenchmarkArtifactsData_Unmarshal_V2(b *testing.B) {
	raw := loadFileData(b, "ttbb-data/ei-afx-data.json")
	data := []byte(strings.ReplaceAll(string(raw), "./data.schema.json", "./ttbb-data/data.schema.json"))
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var s Store
		if err := jsonv2.Unmarshal(data, &s); err != nil {
			b.Fatalf("v2 unmarshal failed: %v", err)
		}
	}
}

func BenchmarkArtifactsData_Marshal_V1(b *testing.B) {
	raw := loadFileData(b, "ttbb-data/ei-afx-data.json")
	data := []byte(strings.ReplaceAll(string(raw), "./data.schema.json", "./ttbb-data/data.schema.json"))
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		b.Fatalf("setup failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	var totalBytes int64
	for i := 0; i < b.N; i++ {
		out, err := json.Marshal(s)
		if err != nil {
			b.Fatalf("v1 marshal failed: %v", err)
		}
		totalBytes = int64(len(out))
	}
	b.SetBytes(totalBytes)
}

func BenchmarkArtifactsData_Marshal_V2(b *testing.B) {
	raw := loadFileData(b, "ttbb-data/ei-afx-data.json")
	data := []byte(strings.ReplaceAll(string(raw), "./data.schema.json", "./ttbb-data/data.schema.json"))
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		b.Fatalf("setup failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	var totalBytes int64
	for i := 0; i < b.N; i++ {
		out, err := jsonv2.Marshal(s)
		if err != nil {
			b.Fatalf("v2 marshal failed: %v", err)
		}
		totalBytes = int64(len(out))
	}
	b.SetBytes(totalBytes)
}

// -------------------------------------------------------------
// 4. Stream Decoding (Decoder / UnmarshalRead)
// -------------------------------------------------------------

func BenchmarkContracts_StreamDecode_V1(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-contracts.json")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		dec := json.NewDecoder(r)
		var contracts []EggIncContract
		if err := dec.Decode(&contracts); err != nil {
			b.Fatalf("v1 stream decode failed: %v", err)
		}
	}
}

func BenchmarkContracts_StreamDecode_V2(b *testing.B) {
	data := loadFileData(b, "ttbb-data/ei-contracts.json")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		var contracts []EggIncContract
		if err := jsonv2.UnmarshalRead(r, &contracts); err != nil {
			b.Fatalf("v2 stream decode failed: %v", err)
		}
	}
}
