package memorygraph

import (
	"testing"
	"time"

	"nukara/backend/internal/store"
)

func TestRecall_BuildsBoundedActivationSet(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	seed := store.TemporalMemoryNode{
		ID:         "episode-seed",
		NodeType:   "episode",
		Title:      "昨晚散步",
		Summary:    "昨晚下班后去江边散步，心情放松。",
		Status:     "active",
		OccurredAt: now.Add(-2 * time.Hour),
		Salience:   0.72,
	}
	olderEpisode := store.TemporalMemoryNode{
		ID:         "episode-older",
		NodeType:   "episode",
		Title:      "前晚散步",
		Summary:    "前晚也去散步了。",
		Status:     "active",
		OccurredAt: now.Add(-26 * time.Hour),
		Salience:   0.58,
	}
	promise := store.TemporalMemoryNode{
		ID:         "promise-open",
		NodeType:   "promise",
		Title:      "继续保持散步",
		Summary:    "答应自己这周继续保持凌晨散步。",
		Status:     "active",
		OccurredAt: now.Add(-20 * time.Hour),
		Salience:   0.7,
	}
	unrelated := store.TemporalMemoryNode{
		ID:         "unrelated",
		NodeType:   "episode",
		Title:      "午饭",
		Summary:    "中午吃了拉面。",
		Status:     "active",
		OccurredAt: now.Add(-90 * time.Minute),
		Salience:   0.1,
	}

	activated := BuildActivationSet(
		Cue{QueryText: "你最近凌晨散步怎么样了", EntityHints: []string{"散步"}},
		[]store.TemporalMemoryNode{seed},
		[]store.TemporalMemoryNode{seed, olderEpisode, promise, unrelated},
		[]store.TemporalMemoryEdge{
			{SourceID: seed.ID, TargetID: olderEpisode.ID, EdgeType: "follows", Weight: 0.95, Status: "active"},
			{SourceID: seed.ID, TargetID: promise.ID, EdgeType: "supports", Weight: 0.9, Status: "active"},
		},
		ActivationOptions{Limit: 3, MaxDepth: 2, Now: now},
		nil,
	)

	if len(activated) != 3 {
		t.Fatalf("expected bounded activation size 3, got %d", len(activated))
	}
	assertActivatedContains(t, activated, seed.ID)
	assertActivatedContains(t, activated, olderEpisode.ID)
	assertActivatedContains(t, activated, promise.ID)
	assertActivatedNotContains(t, activated, unrelated.ID)

	chains := BuildRecallChains(activated, []store.TemporalMemoryEdge{{SourceID: seed.ID, TargetID: olderEpisode.ID, EdgeType: "follows", Weight: 0.95, Status: "active"}})
	if len(chains) == 0 {
		t.Fatal("expected at least one recall chain")
	}
	if got := chains[0].Timeline[0].ID; got != olderEpisode.ID {
		t.Fatalf("expected chain to start from older episode, got %q", got)
	}
	if got := chains[0].Timeline[len(chains[0].Timeline)-1].ID; got != seed.ID {
		t.Fatalf("expected chain to end at seed episode, got %q", got)
	}
}

func TestRecall_PrefersOpenLoopsUntilResolved(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	openPromise := store.TemporalMemoryNode{
		ID:         "promise-open",
		NodeType:   "promise",
		Title:      "报名摄影课",
		Summary:    "还没完成报名摄影课这件事。",
		Status:     "active",
		OccurredAt: now.Add(-96 * time.Hour),
		Salience:   0.55,
	}
	resolvedPromise := store.TemporalMemoryNode{
		ID:         "promise-resolved",
		NodeType:   "promise",
		Title:      "买胶片",
		Summary:    "已经买完胶片了。",
		Status:     "resolved",
		OccurredAt: now.Add(-24 * time.Hour),
		Salience:   0.8,
	}
	staleSnapshot := store.TemporalMemoryNode{
		ID:         "state-stale",
		NodeType:   "state_snapshot",
		Title:      "前天状态",
		Summary:    "前天有点累。",
		Status:     "active",
		OccurredAt: now.Add(-48 * time.Hour),
		Salience:   0.7,
	}

	activated := BuildActivationSet(
		Cue{QueryText: "你记得我还要做什么吗", EntityHints: []string{"报名", "摄影课"}},
		[]store.TemporalMemoryNode{openPromise, resolvedPromise, staleSnapshot},
		[]store.TemporalMemoryNode{openPromise, resolvedPromise, staleSnapshot},
		nil,
		ActivationOptions{Limit: 2, MaxDepth: 1, Now: now},
		nil,
	)

	if len(activated) == 0 {
		t.Fatal("expected activation results")
	}
	if got := activated[0].Node.ID; got != openPromise.ID {
		t.Fatalf("expected open promise to rank first, got %q", got)
	}
	if len(activated) > 1 && activated[1].Node.ID == resolvedPromise.ID && activated[1].Score >= activated[0].Score {
		t.Fatalf("expected resolved promise to rank below open promise")
	}
}

func assertActivatedContains(t *testing.T, activated []ActivatedNode, id string) {
	t.Helper()
	for _, item := range activated {
		if item.Node.ID == id {
			return
		}
	}
	t.Fatalf("expected activation set to contain %q", id)
}

func assertActivatedNotContains(t *testing.T, activated []ActivatedNode, id string) {
	t.Helper()
	for _, item := range activated {
		if item.Node.ID == id {
			t.Fatalf("expected activation set not to contain %q", id)
		}
	}
}
