package leaderboard

import (
	"sync"
	"testing"
	"time"
)

func TestPostProgress_ThreadSafety(t *testing.T) {
	prog := &PostProgress{
		TotalMetrics: 100,
	}

	var wg sync.WaitGroup
	workers := 10
	metricsPerWorker := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < metricsPerWorker; j++ {
				prog.recordMetric(10 * time.Millisecond)
				_, _, _ = prog.snapshot()
			}
		}()
	}

	wg.Wait()

	posted, total, avg := prog.snapshot()
	if posted != 100 {
		t.Fatalf("expected 100 posted metrics, got %d", posted)
	}
	if total != 100 {
		t.Fatalf("expected total metrics 100, got %d", total)
	}
	if avg < 10*time.Millisecond {
		t.Fatalf("expected average duration around 10ms, got %v", avg)
	}
}

func TestPostProgress_NilSafety(t *testing.T) {
	var prog *PostProgress

	posted, total, avg := prog.recordMetric(time.Second)
	if posted != 0 || total != 0 || avg != 0 {
		t.Fatalf("expected zeroes for nil PostProgress recordMetric, got (%d, %d, %v)", posted, total, avg)
	}

	posted, total, avg = prog.snapshot()
	if posted != 0 || total != 0 || avg != 0 {
		t.Fatalf("expected zeroes for nil PostProgress snapshot, got (%d, %d, %v)", posted, total, avg)
	}
}

func TestResolveGuildName(t *testing.T) {
	if resolveGuildName(nil, "") != "" {
		t.Fatalf("expected empty string for empty guild ID")
	}
	if resolveGuildName(nil, "unknown-id") != "unknown-id" {
		t.Fatalf("expected fallback to guild ID when client is nil")
	}
}
