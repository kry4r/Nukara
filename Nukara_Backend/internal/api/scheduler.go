package api

import (
	"context"
	"log"
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

// frequencyCooldown maps frequency setting to minimum hours between proactive messages.
var frequencyCooldown = map[string]time.Duration{
	"high":   2 * time.Hour,
	"normal": 4 * time.Hour,
	"low":    8 * time.Hour,
}

// triggerWindow defines a time-of-day window for a trigger type.
type triggerWindow struct {
	TriggerType string
	StartHour   int
	EndHour     int
}

var triggerWindows = []triggerWindow{
	{TriggerType: "morning_greeting", StartHour: 8, EndHour: 9},
	{TriggerType: "evening_greeting", StartHour: 21, EndHour: 22},
}

const inactivityThreshold = 4 * time.Hour

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

	cooldown := frequencyCooldown[settings.Frequency]
	if cooldown == 0 {
		cooldown = frequencyCooldown["normal"]
	}

	// Check recent proactive logs to enforce cooldown.
	recentLogs := ps.server.store.ListProactiveLogs(userID, 1)
	if len(recentLogs) > 0 && now.Sub(recentLogs[0].CreatedAt) < cooldown {
		return
	}

	conversations := ps.server.store.ListConversations(userID)
	if len(conversations) == 0 {
		return
	}

	// Determine trigger type based on current time.
	triggerType := ps.detectTrigger(now, conversations)
	if triggerType == "" {
		return
	}

	// Pick the most recent conversation to send the proactive message to.
	conv := conversations[0]
	bot, found := ps.server.store.GetBot(userID, conv.BotID)
	if !found {
		return
	}

	ps.server.sendProactiveMessage(userID, bot, conv, triggerType)
}

func (ps *proactiveScheduler) detectTrigger(now time.Time, conversations []store.Conversation) string {
	hour := now.Hour()

	// Check time-based triggers.
	for _, tw := range triggerWindows {
		if hour >= tw.StartHour && hour < tw.EndHour {
			return tw.TriggerType
		}
	}

	// Check inactivity trigger (during active hours 9:00-21:00).
	if hour >= 9 && hour < 21 && len(conversations) > 0 {
		lastActivity := conversations[0].LastMessageAt
		if now.Sub(lastActivity) >= inactivityThreshold {
			return "inactivity"
		}
	}

	return ""
}

// sendProactiveMessage generates and delivers a proactive message for a user+bot pair.
// Reused by both the scheduler and the manual trigger API.
func (s *Server) sendProactiveMessage(userID string, bot store.Bot, conv store.Conversation, triggerType string) {
	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	message, err := s.agent.Proactive(context.Background(), convID, "proactive", triggerType)
	if err != nil {
		log.Printf("[scheduler] agent.Proactive failed: %v", err)
	}
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
		EmotionTag:     "gentle",
	})
	if !ok {
		return
	}

	status := selectBotStatus("gentle", conv.ID)
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
