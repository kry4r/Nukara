package api

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/store"
)

// proactiveScheduler periodically scans users and triggers proactive messages
// based on time windows (morning/evening) and inactivity detection.
type proactiveScheduler struct {
	server   *Server
	interval time.Duration
	stop     chan struct{}
}

// frequencyCooldown maps frequency setting to minimum time between proactive messages.
// Override all with NUKARA_PROACTIVE_COOLDOWN env var (e.g. "5m" for dev testing).
var frequencyCooldown = map[string]time.Duration{
	"high":   2 * time.Hour,
	"normal": 4 * time.Hour,
	"low":    8 * time.Hour,
}

func getFrequencyCooldown(freq string) time.Duration {
	if v := os.Getenv("NUKARA_PROACTIVE_COOLDOWN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	cd := frequencyCooldown[freq]
	if cd == 0 {
		cd = frequencyCooldown["normal"]
	}
	return cd
}

// triggerWindow defines a time-of-day window for a trigger type.
type triggerWindow struct {
	TriggerType string
	StartHour   int
	EndHour     int
}

var triggerWindows = []triggerWindow{
	{TriggerType: "morning_care", StartHour: 7, EndHour: 9},
	{TriggerType: "evening_care", StartHour: 21, EndHour: 22},
	{TriggerType: "random_share", StartHour: 10, EndHour: 20},
}

// perTriggerCooldown prevents the same trigger type from firing too often.
var perTriggerCooldown = map[string]time.Duration{
	"morning_care":            20 * time.Hour, // once per day
	"evening_care":            20 * time.Hour,
	"curiosity_after_silence": 1 * time.Hour,
	"worry_after_long_silence": 4 * time.Hour,
	"random_share":            3 * time.Hour,
}

// inactivityThreshold returns the duration after which a user is considered inactive.
// Configurable via NUKARA_INACTIVITY_THRESHOLD env var (e.g. "3m", "4h"). Default: 3m.
func inactivityThreshold() time.Duration {
	if v := os.Getenv("NUKARA_INACTIVITY_THRESHOLD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 3 * time.Minute
}

func newScheduler(server *Server, interval time.Duration) *proactiveScheduler {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &proactiveScheduler{
		server:   server,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (ps *proactiveScheduler) Start() {
	go ps.loop()
	log.Printf("[scheduler] started, interval=%s", ps.interval)
}

func (ps *proactiveScheduler) Stop() {
	close(ps.stop)
}

func (ps *proactiveScheduler) loop() {
	ticker := time.NewTicker(ps.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ps.stop:
			log.Printf("[scheduler] stopped")
			return
		case <-ticker.C:
			ps.tick()
		}
	}
}

func (ps *proactiveScheduler) tick() {
	now := time.Now()
	userIDs := ps.server.store.ListAllUserIDs()
	for _, userID := range userIDs {
		ps.processUser(userID, now)
	}
}

func (ps *proactiveScheduler) processUser(userID string, now time.Time) {
	settings := ps.server.store.GetNotificationSettings(userID)
	if !settings.ProactiveEnabled {
		return
	}
	if isDND(settings, now) {
		return
	}

	cooldown := getFrequencyCooldown(settings.Frequency)

	// Check recent proactive logs to enforce cooldown.
	recentLogs := ps.server.store.ListProactiveLogs(userID, 1)
	if len(recentLogs) > 0 && now.Sub(recentLogs[0].CreatedAt) < cooldown {
		return
	}

	conversations := ps.server.store.ListConversations(userID)
	if len(conversations) == 0 {
		return
	}

	// Back-off: if user hasn't responded since last proactive message, increase cooldown.
	// Check the most recent conversation's messages to see if user replied.
	conv := conversations[0]
	if len(recentLogs) > 0 {
		msgs, _ := ps.server.store.ListMessages(userID, conv.ID, 10)
		unanswered := 0
		for _, m := range msgs {
			if m.SenderType == "user" {
				break
			}
			if m.IsProactive {
				unanswered++
			}
		}
		// Exponential back-off: 2 unanswered = 2x cooldown, 3 = 4x, etc.
		if unanswered >= 2 {
			backoff := cooldown * time.Duration(1<<(unanswered-1))
			if now.Sub(recentLogs[0].CreatedAt) < backoff {
				return
			}
		}
	}

	// Determine trigger type based on current time.
	triggerType := ps.detectTrigger(now, conversations)
	if triggerType == "" {
		return
	}

	// Per-trigger cooldown: check if this specific trigger fired too recently.
	if cd, ok := perTriggerCooldown[triggerType]; ok {
		recentLogs := ps.server.store.ListProactiveLogs(userID, 10)
		for _, lg := range recentLogs {
			if lg.TriggerType == triggerType && now.Sub(lg.CreatedAt) < cd {
				return
			}
		}
	}

	bot, found := ps.server.store.GetBot(userID, conv.BotID)
	if !found {
		return
	}

	ps.server.sendProactiveMessage(userID, bot, conv, triggerType)
}

func (ps *proactiveScheduler) detectTrigger(now time.Time, conversations []store.Conversation) string {
	hour := now.Hour()

	// Check time-based triggers (morning/evening care).
	for _, tw := range triggerWindows {
		if tw.TriggerType == "random_share" {
			continue // handled separately below
		}
		if hour >= tw.StartHour && hour < tw.EndHour {
			return tw.TriggerType
		}
	}

	// Check inactivity triggers (curiosity vs worry based on gap duration).
	threshold := inactivityThreshold()
	longThreshold := 2 * time.Hour
	inActiveHours := hour >= 9 && hour < 21

	if len(conversations) > 0 && (inActiveHours || threshold < time.Hour) {
		gap := now.Sub(conversations[0].LastMessageAt)
		if gap >= longThreshold {
			return "worry_after_long_silence"
		}
		if gap >= threshold {
			return "curiosity_after_silence"
		}
	}

	// Random share during daytime active hours (10-20).
	if hour >= 10 && hour < 20 {
		// Use minute-based deterministic check so it doesn't fire every tick.
		// Fires roughly once per hour window when cooldown allows.
		if now.Minute() < 5 {
			return "random_share"
		}
	}

	return ""
}

// sendProactiveMessage generates and delivers a proactive message for a user+bot pair.
// Reused by both the scheduler and the manual trigger API.
func (s *Server) sendProactiveMessage(userID string, bot store.Bot, conv store.Conversation, triggerType string) {
	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	sysCtx := agent.BuildSystemContext(bot, nil)

	// Inject emotion context if available
	if emoCtx, ok := s.store.GetEmotionContext(userID, bot.ID); ok {
		sysCtx["emotion_trend"] = emoCtx.EmotionTrend
		sysCtx["last_tone"] = emoCtx.LastTone
	}

	message, err := s.agent.Proactive(context.Background(), convID, "proactive", triggerType, sysCtx)
	if err != nil {
		log.Printf("[scheduler] agent.Proactive failed: %v", err)
	}
	// Apply the same sanitization pipeline as regular chat messages.
	message = agent.SanitizeLLMReply(message)
	message, _, _ = agent.ExtractStatus(message, "")
	message, emotion := agent.ExtractEmotion(message)
	if strings.TrimSpace(message) == "" {
		log.Printf("[scheduler] empty proactive message for user=%s bot=%s trigger=%s", userID, bot.ID, triggerType)
		return
	}

	storedMessage, ok := s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "bot",
		ContentType:    "text",
		Content:        store.MessageContent{Type: "text", Text: message},
		IsProactive:    true,
		EmotionTag:     emotion,
	})
	if !ok {
		return
	}

	status := selectBotStatus(emotion, conv.ID)
	s.store.SaveBotStatus(userID, bot.ID, status.Emoji, status.Text)

	sentByWS := s.wsHub.publishToUser(userID, wsProactiveEvent(storedMessage)) > 0
	if sentByWS {
		s.wsHub.publishToUser(userID, wsBotStatusEvent(conv.ID, status.Emoji, status.Text))
	}

	sentAPNs := false
	if !sentByWS {
		deviceToken, hasToken := s.store.GetDeviceToken(userID)
		if hasToken {
			if err := s.apns.Send(deviceToken.Token, bot.Name, message, conv.ID); err == nil {
				sentAPNs = true
			}
		}
	}

	s.store.AddProactiveLog(store.ProactiveLog{
		UserID:         userID,
		ConversationID: conv.ID,
		BotID:          bot.ID,
		TriggerType:    triggerType,
		Message:        message,
		SentByWS:       sentByWS,
		SentByAPNs:     sentAPNs,
	})

	log.Printf("[scheduler] sent proactive message user=%s bot=%s trigger=%s ws=%t apns=%t", userID, bot.ID, triggerType, sentByWS, sentAPNs)
}

// StartScheduler launches the proactive message scheduler in the background.
func (s *Server) StartScheduler(interval time.Duration) {
	sched := newScheduler(s, interval)
	sched.Start()
}

// isDND checks if the current time falls within the user's Do Not Disturb window.
func isDND(settings store.NotificationSettings, now time.Time) bool {
	start := parseHHMM(settings.DNDStart)
	end := parseHHMM(settings.DNDEnd)
	if start < 0 || end < 0 {
		return false
	}
	current := now.Hour()*60 + now.Minute()
	if start <= end {
		return current >= start && current < end
	}
	// Crosses midnight, e.g. 23:00 - 07:00.
	return current >= start || current < end
}

// parseHHMM parses "HH:MM" into minutes since midnight. Returns -1 on failure.
func parseHHMM(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return -1
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h, m := 0, 0
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return -1
		}
		h = h*10 + int(c-'0')
	}
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return -1
		}
		m = m*10 + int(c-'0')
	}
	if h > 23 || m > 59 {
		return -1
	}
	return h*60 + m
}
