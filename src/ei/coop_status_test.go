package ei

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

type rewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.base == nil {
		return nil, errors.New("nil base transport")
	}
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = rt.target.Scheme
	req2.URL.Host = rt.target.Host
	req2.Host = rt.target.Host
	return rt.base.RoundTrip(req2)
}

func coopStatusSuccessBody(t *testing.T) []byte {
	t.Helper()
	statusPayload, err := proto.Marshal(&ContractCoopStatusResponse{})
	if err != nil {
		t.Fatalf("marshal ContractCoopStatusResponse: %v", err)
	}
	authPayload, err := proto.Marshal(&AuthenticatedMessage{Message: statusPayload})
	if err != nil {
		t.Fatalf("marshal AuthenticatedMessage: %v", err)
	}
	return []byte(base64.StdEncoding.EncodeToString(authPayload))
}

func setTestCwd(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func setAuxbrainServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = rewriteTransport{target: targetURL, base: originalTransport}
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
}

func setCoopStatusFixEnabled(t *testing.T, enabled bool) {
	t.Helper()
	previous := CoopStatusFixEnabled
	CoopStatusFixEnabled = func() bool { return enabled }
	t.Cleanup(func() {
		CoopStatusFixEnabled = previous
	})
}

func resetCoopStatusCache(t *testing.T) {
	t.Helper()
	previous := eiDatas
	eiDatas = make(map[string]*eiData)
	t.Cleanup(func() {
		eiDatas = previous
	})
}

func TestGetCoopStatus_RetriesOn500NonEOP(t *testing.T) {
	setTestCwd(t)
	resetCoopStatusCache(t)
	setCoopStatusFixEnabled(t, true)

	validBody := coopStatusSuccessBody(t)
	paths := make([]string, 0, 2)
	calls := 0
	setAuxbrainServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		paths = append(paths, r.URL.Path)
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("crypyo_key"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(validBody)
	})

	_, _, _, err := GetCoopStatus("contract-a", "coop-a", "EI1234567890123456")
	if err != nil {
		t.Fatalf("GetCoopStatus returned unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (initial + retry), got %d", calls)
	}
	if len(paths) != 2 || paths[0] != "/ei/coop_status_bot" || paths[1] != "/ei/coop_status" {
		t.Fatalf("unexpected request paths: %v", paths)
	}
}

func TestGetCoopStatus_DoesNotRetryOn500EOP(t *testing.T) {
	setTestCwd(t)
	resetCoopStatusCache(t)
	setCoopStatusFixEnabled(t, true)

	paths := make([]string, 0, 2)
	calls := 0
	setAuxbrainServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("eop"))
	})

	_, _, _, err := GetCoopStatus("contract-b", "coop-b", "EI1234567890123456")
	if err == nil {
		t.Fatal("expected error for 500 eop response, got nil")
	}
	if !strings.Contains(err.Error(), "API Error Code: 500") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry on eop), got %d", calls)
	}
	if len(paths) != 1 || paths[0] != "/ei/coop_status_bot" {
		t.Fatalf("unexpected request paths: %v", paths)
	}
}

func TestGetCoopStatusUncached_BypassesMemoryCache(t *testing.T) {
	setTestCwd(t)
	resetCoopStatusCache(t)
	setCoopStatusFixEnabled(t, true)

	validBody := coopStatusSuccessBody(t)
	calls := 0
	setAuxbrainServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(validBody)
	})

	_, _, _, err := GetCoopStatus("contract-cache", "coop-cache", "EI1234567890123456")
	if err != nil {
		t.Fatalf("GetCoopStatus returned unexpected error: %v", err)
	}
	_, _, _, err = GetCoopStatus("contract-cache", "coop-cache", "EI1234567890123456")
	if err != nil {
		t.Fatalf("second GetCoopStatus returned unexpected error: %v", err)
	}
	_, _, _, err = GetCoopStatusUncached("contract-cache", "coop-cache", "EI1234567890123456")
	if err != nil {
		t.Fatalf("GetCoopStatusUncached returned unexpected error: %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected 2 network calls with uncached fetch, got %d", calls)
	}
}

func TestGetCoopStatusForCompletedContracts_RetriesOn500NonEOP(t *testing.T) {
	setTestCwd(t)
	setCoopStatusFixEnabled(t, true)

	validBody := coopStatusSuccessBody(t)
	paths := make([]string, 0, 2)
	calls := 0
	setAuxbrainServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		paths = append(paths, r.URL.Path)
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("crypyo_key"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(validBody)
	})

	_, _, _, err := GetCoopStatusForCompletedContracts("contract-c", "coop-c", "EI1234567890123456")
	if err != nil {
		t.Fatalf("GetCoopStatusForCompletedContracts returned unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (initial + retry), got %d", calls)
	}
	if len(paths) != 2 || paths[0] != "/ei/coop_status_bot" || paths[1] != "/ei/coop_status" {
		t.Fatalf("unexpected request paths: %v", paths)
	}
}

func TestGetCoopStatusForCompletedContracts_DoesNotRetryOn500EOP(t *testing.T) {
	setTestCwd(t)
	setCoopStatusFixEnabled(t, true)

	paths := make([]string, 0, 2)
	calls := 0
	setAuxbrainServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("eop"))
	})

	_, _, _, err := GetCoopStatusForCompletedContracts("contract-d", "coop-d", "EI1234567890123456")
	if err == nil {
		t.Fatal("expected error for 500 eop response, got nil")
	}
	if !strings.Contains(err.Error(), "API Error Code: 500") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry on eop), got %d", calls)
	}
	if len(paths) != 1 || paths[0] != "/ei/coop_status_bot" {
		t.Fatalf("unexpected request paths: %v", paths)
	}
}
