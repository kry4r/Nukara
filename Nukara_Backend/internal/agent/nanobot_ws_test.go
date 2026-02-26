package agent

import (
	"fmt"
	"testing"
)

func TestWSPoolPickConsistency(t *testing.T) {
	pool := &nanobotWSPool{
		clients: make([]*nanobotWSClient, 4),
		size:    4,
	}
	for i := range pool.clients {
		pool.clients[i] = &nanobotWSClient{}
	}

	convID := "nukara:user1:bot1:conv1"
	first := pool.pick(convID)
	for i := 0; i < 100; i++ {
		if got := pool.pick(convID); got != first {
			t.Fatalf("pick(%q) returned different client on iteration %d", convID, i)
		}
	}
}

func TestWSPoolPickDistribution(t *testing.T) {
	poolSize := 4
	pool := &nanobotWSPool{
		clients: make([]*nanobotWSClient, poolSize),
		size:    poolSize,
	}
	for i := range pool.clients {
		pool.clients[i] = &nanobotWSClient{}
	}

	counts := make(map[int]int)
	n := 10000
	for i := 0; i < n; i++ {
		// Use realistic conversation IDs with varied user/bot/conv components
		convID := fmt.Sprintf("nukara:user_%d:bot_%d:conv_%d", i%200, i%10, i)
		picked := pool.pick(convID)
		for j, c := range pool.clients {
			if c == picked {
				counts[j]++
				break
			}
		}
	}

	// With 10000 samples across 4 buckets, expect ~2500 each; allow ±40%
	for idx, count := range counts {
		if count < 1000 || count > 5000 {
			t.Errorf("client %d got %d picks out of %d (too skewed)", idx, count, n)
		}
	}
}

func TestWSPoolPickDifferentConvIDs(t *testing.T) {
	pool := &nanobotWSPool{
		clients: make([]*nanobotWSClient, 4),
		size:    4,
	}
	for i := range pool.clients {
		pool.clients[i] = &nanobotWSClient{}
	}

	// Use diverse IDs to increase chance of hitting different buckets
	ids := []string{
		"nukara:alice:bot_a:conv_100",
		"nukara:bob:bot_b:conv_200",
		"nukara:charlie:bot_c:conv_300",
		"nukara:dave:bot_d:conv_400",
		"nukara:eve:bot_e:conv_500",
		"nukara:frank:bot_f:conv_600",
		"nukara:grace:bot_g:conv_700",
		"nukara:heidi:bot_h:conv_800",
	}

	seen := make(map[*nanobotWSClient]bool)
	for _, id := range ids {
		seen[pool.pick(id)] = true
	}

	if len(seen) < 2 {
		t.Errorf("expected at least 2 different clients for %d convIDs, got %d", len(ids), len(seen))
	}
}
