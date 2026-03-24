// Package memorybuffer implements delayed aggregation of conversation turns
// before memory extraction. Instead of extracting memory after every single
// turn, the buffer accumulates turns for the same topic and flushes the whole
// batch when the topic changes or a time/size window is exceeded.
package memorybuffer

import (
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxTurns   = 5
	defaultMaxAge     = 10 * time.Minute
	defaultMaxChars   = 1200
)

// TurnEntry represents one buffered conversation turn.
type TurnEntry struct {
	TurnID   string
	UserText string
	BotText  string
	AddedAt  time.Time
}

// ConversationBuffer holds buffered turns for a single conversation.
type ConversationBuffer struct {
	ConversationID string
	TopicID        string
	TopicKeywords  []string
	Turns          []TurnEntry
	StartedAt      time.Time
	LastUpdatedAt  time.Time
}

// FlushReason indicates why a flush was triggered.
type FlushReason string

const (
	FlushReasonTopicChange FlushReason = "topic_change"
	FlushReasonMaxTurns    FlushReason = "max_turns"
	FlushReasonMaxAge      FlushReason = "max_age"
	FlushReasonMaxChars    FlushReason = "max_chars"
	FlushReasonForced      FlushReason = "forced"
)

// FlushResult is returned when a buffer is flushed.
type FlushResult struct {
	ConversationID string
	TurnIDs        []string
	AggregatedText string // combined userText+botText for all flushed turns
	LastTurnID     string
	Reason         FlushReason
}

// Options configures flush thresholds.
type Options struct {
	// MaxTurns is the maximum number of turns to accumulate before forcing a flush.
	MaxTurns int
	// MaxAge is the maximum time a buffer can accumulate before forcing a flush.
	MaxAge time.Duration
	// MaxChars is the maximum total character count before forcing a flush.
	MaxChars int
}

func (o Options) maxTurns() int {
	if o.MaxTurns <= 0 {
		return defaultMaxTurns
	}
	return o.MaxTurns
}

func (o Options) maxAge() time.Duration {
	if o.MaxAge <= 0 {
		return defaultMaxAge
	}
	return o.MaxAge
}

func (o Options) maxChars() int {
	if o.MaxChars <= 0 {
		return defaultMaxChars
	}
	return o.MaxChars
}

// AppendResult describes what happened when a turn was appended.
type AppendResult struct {
	// Flushed is non-nil if the buffer was flushed due to this append.
	Flushed *FlushResult
	// BufferedCount is how many turns are now in the buffer (after possible flush).
	BufferedCount int
}

// Service manages per-conversation memory buffers.
type Service struct {
	mu      sync.Mutex
	buffers map[string]*ConversationBuffer // key: conversationID
	opts    Options
	now     func() time.Time // injectable for tests
}

// NewService creates a new buffer Service.
func NewService(opts Options) *Service {
	return &Service{
		buffers: make(map[string]*ConversationBuffer),
		opts:    opts,
		now:     time.Now,
	}
}

// Append adds a turn to the buffer for the given conversation.
// If the topic has changed (detected via newTopicKeywords) or a window
// threshold is exceeded, the existing buffer is flushed first and the new
// turn starts a fresh buffer.
// topicContinues=false means the caller detected a topic change.
func (s *Service) Append(
	conversationID, turnID, userText, botText string,
	topicContinues bool,
	newTopicKeywords []string,
) AppendResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	buf, exists := s.buffers[conversationID]

	var flushed *FlushResult

	if exists && len(buf.Turns) > 0 {
		reason, shouldFlush := s.shouldFlush(buf, now, topicContinues)
		if shouldFlush {
			flushed = s.flush(buf, reason)
			buf = nil
			exists = false
		}
	}

	if !exists || buf == nil {
		topicID := generateTopicID(now)
		buf = &ConversationBuffer{
			ConversationID: conversationID,
			TopicID:        topicID,
			TopicKeywords:  append([]string(nil), newTopicKeywords...),
			StartedAt:      now,
			LastUpdatedAt:  now,
		}
		s.buffers[conversationID] = buf
	} else if len(newTopicKeywords) > 0 {
		// Merge new keywords into existing topic
		buf.TopicKeywords = mergeKeywords(buf.TopicKeywords, newTopicKeywords)
	}

	entry := TurnEntry{
		TurnID:   turnID,
		UserText: strings.TrimSpace(userText),
		BotText:  strings.TrimSpace(botText),
		AddedAt:  now,
	}
	buf.Turns = append(buf.Turns, entry)
	buf.LastUpdatedAt = now

	// After appending, check if we've now hit a window threshold.
	// (Topic change and MaxAge are checked before append above; MaxTurns and
	// MaxChars are re-checked here because they depend on the updated counts.)
	if flushed == nil {
		if reason, ok := s.shouldFlushAfterAppend(buf, now); ok {
			flushed = s.flush(buf, reason)
			// Buffer is now cleared; BufferedCount reflects what was flushed (0 remaining).
			return AppendResult{
				Flushed:       flushed,
				BufferedCount: 0,
			}
		}
	}

	return AppendResult{
		Flushed:       flushed,
		BufferedCount: len(buf.Turns),
	}
}

// ForceFlush immediately flushes the buffer for the given conversation.
// Returns nil if there is no buffer or it is empty.
func (s *Service) ForceFlush(conversationID string) *FlushResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	buf, exists := s.buffers[conversationID]
	if !exists || len(buf.Turns) == 0 {
		return nil
	}
	return s.flush(buf, FlushReasonForced)
}

// FlushStale flushes any buffers that have exceeded maxAge.
// Useful for a periodic cleanup goroutine.
func (s *Service) FlushStale() []FlushResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	results := make([]FlushResult, 0)
	for convID, buf := range s.buffers {
		if len(buf.Turns) == 0 {
			delete(s.buffers, convID)
			continue
		}
		age := now.Sub(buf.StartedAt)
		if age >= s.opts.maxAge() {
			result := s.flush(buf, FlushReasonMaxAge)
			results = append(results, *result)
		}
	}
	return results
}

// shouldFlush returns (reason, true) if the buffer should be flushed BEFORE
// a new entry is appended. Only topic-change and age are checked here since
// turn-count and char-count depend on the new entry being present.
func (s *Service) shouldFlush(buf *ConversationBuffer, now time.Time, topicContinues bool) (FlushReason, bool) {
	if !topicContinues {
		return FlushReasonTopicChange, true
	}
	if now.Sub(buf.StartedAt) >= s.opts.maxAge() {
		return FlushReasonMaxAge, true
	}
	return "", false
}

// shouldFlushAfterAppend returns (reason, true) if the buffer should be
// flushed AFTER a new entry has been appended (turn-count and char-count).
func (s *Service) shouldFlushAfterAppend(buf *ConversationBuffer, _ time.Time) (FlushReason, bool) {
	if len(buf.Turns) >= s.opts.maxTurns() {
		return FlushReasonMaxTurns, true
	}
	if totalChars(buf.Turns) >= s.opts.maxChars() {
		return FlushReasonMaxChars, true
	}
	return "", false
}

// flush builds a FlushResult from the buffer and removes it from the map.
// Must be called with s.mu held.
func (s *Service) flush(buf *ConversationBuffer, reason FlushReason) *FlushResult {
	turnIDs := make([]string, 0, len(buf.Turns))
	parts := make([]string, 0, len(buf.Turns)*2)
	lastTurnID := ""
	for _, t := range buf.Turns {
		if t.TurnID != "" {
			turnIDs = append(turnIDs, t.TurnID)
			lastTurnID = t.TurnID
		}
		if t.UserText != "" {
			parts = append(parts, "用户："+t.UserText)
		}
		if t.BotText != "" {
			parts = append(parts, "机器人："+t.BotText)
		}
	}

	delete(s.buffers, buf.ConversationID)
	return &FlushResult{
		ConversationID: buf.ConversationID,
		TurnIDs:        turnIDs,
		AggregatedText: strings.Join(parts, "\n"),
		LastTurnID:     lastTurnID,
		Reason:         reason,
	}
}

// totalChars returns the total rune count of all userText+botText in the buffer.
func totalChars(turns []TurnEntry) int {
	total := 0
	for _, t := range turns {
		total += len([]rune(t.UserText)) + len([]rune(t.BotText))
	}
	return total
}

// generateTopicID creates a simple topic ID based on time.
func generateTopicID(now time.Time) string {
	return "topic-" + now.UTC().Format("20060102150405")
}

// mergeKeywords adds new keywords that don't already appear in existing.
func mergeKeywords(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, k := range existing {
		seen[strings.TrimSpace(k)] = struct{}{}
	}
	out := append([]string(nil), existing...)
	for _, k := range incoming {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	return out
}
