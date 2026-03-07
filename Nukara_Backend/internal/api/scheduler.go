package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
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

// getProactiveCooldown resolves the effective proactive interval for a user.
// Override all with NUKARA_PROACTIVE_COOLDOWN env var (e.g. "5m" for dev testing).
func getProactiveCooldown(settings store.NotificationSettings) time.Duration {
	if v := os.Getenv("NUKARA_PROACTIVE_COOLDOWN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	minutes := settings.ProactiveIntervalMinutes
	if minutes <= 0 {
		minutes = store.DefaultProactiveIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
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
	"morning_care":             20 * time.Hour, // once per day
	"evening_care":             20 * time.Hour,
	"curiosity_after_silence":  3 * time.Hour,
	"worry_after_long_silence": 8 * time.Hour,
	"random_share":             3 * time.Hour,
	"share_personal_moment":    8 * time.Hour,
	"share_interesting_fact":   10 * time.Hour,
	"share_emotion":            12 * time.Hour,
}

// inactivityThreshold returns the duration after which a user is considered inactive.
// Configurable via NUKARA_INACTIVITY_THRESHOLD env var (e.g. "8m", "4h"). Default: 8m.
func inactivityThreshold() time.Duration {
	if v := os.Getenv("NUKARA_INACTIVITY_THRESHOLD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 8 * time.Minute
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

	conversations := ps.server.store.ListConversations(userID)
	if len(conversations) == 0 {
		return
	}
	conv := conversations[0]
	bot, found := ps.server.store.GetBot(userID, conv.BotID)
	if !found {
		return
	}

	locale := InferLocaleContext(bot.LifeContext, now)
	localNow := locale.LocalNow()
	if localNow.IsZero() {
		localNow = now
	}
	if isDND(settings, localNow) {
		return
	}

	cooldown := getProactiveCooldown(settings)

	// Check recent proactive logs to enforce cooldown.
	recentLogs := ps.server.store.ListProactiveLogs(userID, 1)
	if len(recentLogs) > 0 && now.Sub(recentLogs[0].CreatedAt) < cooldown {
		return
	}

	online := ps.server.store.IsUserWSOnline(userID)
	lastUserAt, _ := ps.server.store.GetLastUserMessageAt(userID)

	// Back-off: if user hasn't responded since last proactive message, increase cooldown.
	// Check the most recent conversation's messages to see if user replied.
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
	emotionTrend := ""
	if emoCtx, ok := ps.server.store.GetEmotionContext(userID, conv.BotID); ok {
		emotionTrend = strings.TrimSpace(emoCtx.EmotionTrend)
	}

	// Determine trigger type based on current time.
	triggerType := ps.detectTrigger(localNow, conversations, emotionTrend)
	if triggerType == "" {
		return
	}
	if shouldBlockTriggerForPresence(triggerType, online, lastUserAt, now) {
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

	ps.server.sendProactiveMessage(userID, bot, conv, triggerType)
}

func (ps *proactiveScheduler) detectTrigger(now time.Time, conversations []store.Conversation, emotionTrend string) string {
	hour := now.Hour()
	config := ps.getProactiveConfig()

	// Check time-based triggers (morning/evening care).
	for _, tw := range triggerWindows {
		if tw.TriggerType == "random_share" {
			continue // handled separately below
		}
		if hour >= tw.StartHour && hour < tw.EndHour {
			if isMessageTypeEnabled(config, tw.TriggerType) {
				return tw.TriggerType
			}
		}
	}

	// Check inactivity triggers (curiosity vs worry based on gap duration).
	threshold := inactivityThreshold()
	longThreshold := 3 * time.Hour
	inActiveHours := hour >= 9 && hour < 21

	if len(conversations) > 0 && (inActiveHours || threshold < time.Hour) {
		gap := now.Sub(conversations[0].LastMessageAt)
		if gap >= longThreshold {
			if strings.EqualFold(emotionTrend, "negative") && isMessageTypeEnabled(config, "worry_after_long_silence") {
				return "worry_after_long_silence"
			}
			if isMessageTypeEnabled(config, "curiosity_after_silence") {
				return "curiosity_after_silence"
			}
		}
		if gap >= threshold {
			if isMessageTypeEnabled(config, "curiosity_after_silence") {
				return "curiosity_after_silence"
			}
		}
	}

	// Share triggers - new proactive message types
	// share_personal_moment: bot shares personal moments (10:00-20:00, 30% chance)
	if hour >= 10 && hour < 20 && isMessageTypeEnabled(config, "share_personal_moment") {
		if rand.Float32() < 0.3 {
			return "share_personal_moment"
		}
	}

	// share_interesting_fact: bot shares interesting facts (12:00-18:00, 25% chance)
	if hour >= 12 && hour < 18 && isMessageTypeEnabled(config, "share_interesting_fact") {
		if rand.Float32() < 0.25 {
			return "share_interesting_fact"
		}
	}

	// share_emotion: bot shares emotions/thoughts (14:00-21:00, 20% chance)
	if hour >= 14 && hour < 21 && isMessageTypeEnabled(config, "share_emotion") {
		if rand.Float32() < 0.2 {
			return "share_emotion"
		}
	}

	// Random share during daytime active hours (10-20).
	if hour >= 10 && hour < 20 && isMessageTypeEnabled(config, "random_share") {
		// Use minute-based deterministic check so it doesn't fire every tick.
		// Fires roughly once per hour window when cooldown allows.
		if now.Minute() < 5 {
			return "random_share"
		}
	}

	return ""
}

func isNudgeTrigger(trigger string) bool {
	switch trigger {
	case "curiosity_after_silence", "worry_after_long_silence":
		return true
	default:
		return false
	}
}

func isShareTrigger(trigger string) bool {
	switch trigger {
	case "random_share", "share_personal_moment", "share_interesting_fact", "share_emotion":
		return true
	default:
		return false
	}
}

func shouldBlockTriggerForPresence(trigger string, online bool, lastUserAt, now time.Time) bool {
	if !online {
		return false
	}
	if isNudgeTrigger(trigger) {
		return true
	}
	if isShareTrigger(trigger) && !lastUserAt.IsZero() && now.Sub(lastUserAt) < 3*time.Minute {
		return true
	}
	return false
}

// sendProactiveMessage generates and delivers a proactive message for a user+bot pair.
// Reused by both the scheduler and the manual trigger API.
func (s *Server) sendProactiveMessage(userID string, bot store.Bot, conv store.Conversation, triggerType string) {
	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	sysCtx := agent.BuildSystemContext(bot, nil)
	now := time.Now().UTC()

	// Inject emotion context if available
	if emoCtx, ok := s.store.GetEmotionContext(userID, bot.ID); ok {
		sysCtx["emotion_trend"] = emoCtx.EmotionTrend
		sysCtx["last_tone"] = emoCtx.LastTone
	}
	if lastUserAt, ok := s.store.GetLastUserMessageAt(userID); ok && !lastUserAt.IsZero() {
		sysCtx["last_user_message_at"] = lastUserAt.UTC().Format(time.RFC3339)
		sysCtx["time_since_last_user_message"] = now.Sub(lastUserAt).Round(time.Minute).String()
	}
	if lastText, ok := s.lastUserText(userID, conv.ID, 30); ok {
		sysCtx["last_user_message"] = lastText
	}

	message, emotion, _, _, err := s.runRuntimeProactive(context.Background(), userID, bot.ID, convID, triggerType, sysCtx)
	if err != nil {
		log.Printf("[scheduler] runtime proactive failed: %v", err)
		message = fmt.Sprintf("%s：刚想到你了，最近怎么样？", bot.Name)
		emotion = "gentle"
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

func (s *Server) lastUserText(userID, conversationID string, limit int) (string, bool) {
	messages, ok := s.store.ListMessages(userID, conversationID, limit)
	if !ok {
		return "", false
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.SenderType != "user" {
			continue
		}
		text := strings.TrimSpace(msg.Content.Text)
		if text == "" {
			switch msg.Content.Type {
			case "image":
				text = "[图片]"
			case "location":
				if strings.TrimSpace(msg.Content.Name) != "" {
					text = "📍" + strings.TrimSpace(msg.Content.Name)
				} else {
					text = "📍位置"
				}
			}
		}
		if text != "" {
			return text, true
		}
	}
	return "", false
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

// ProactiveConfig represents system-level proactive message configuration
type ProactiveConfig struct {
	Enabled             bool     `json:"enabled"`
	CheckInterval       string   `json:"check_interval"`
	InactivityThreshold string   `json:"inactivity_threshold"`
	Cooldown            string   `json:"cooldown"`
	TimeWindowStart     string   `json:"time_window_start"`
	TimeWindowEnd       string   `json:"time_window_end"`
	EnabledMessageTypes []string `json:"enabled_message_types"`
}

// getProactiveConfig reads configuration from database
func (ps *proactiveScheduler) getProactiveConfig() ProactiveConfig {
	// Try to read from database if PostgresStore is available
	if pgStore, ok := ps.server.store.(*store.PostgresStore); ok {
		var config ProactiveConfig
		var enabledTypesJSON []byte

		err := pgStore.DB().QueryRow(`
			SELECT enabled, check_interval, inactivity_threshold, cooldown,
			       time_window_start, time_window_end, enabled_message_types
			FROM proactive_config LIMIT 1
		`).Scan(&config.Enabled, &config.CheckInterval, &config.InactivityThreshold,
			&config.Cooldown, &config.TimeWindowStart, &config.TimeWindowEnd,
			&enabledTypesJSON)

		if err == nil {
			// Parse JSON array
			if len(enabledTypesJSON) > 0 {
				var types []string
				if err := json.Unmarshal(enabledTypesJSON, &types); err == nil {
					config.EnabledMessageTypes = types
				}
			}
			return config
		}
	}

	// Return default configuration if database read fails or not using PostgresStore
	return defaultProactiveConfig()
}

// defaultProactiveConfig returns default configuration
func defaultProactiveConfig() ProactiveConfig {
	return ProactiveConfig{
		Enabled:             true,
		CheckInterval:       "5m",
		InactivityThreshold: "30m",
		Cooldown:            "60m",
		TimeWindowStart:     "08:00",
		TimeWindowEnd:       "22:00",
		EnabledMessageTypes: []string{
			"morning_care",
			"evening_care",
			"curiosity_after_silence",
			"worry_after_long_silence",
			"random_share",
			"share_personal_moment",
			"share_interesting_fact",
			"share_emotion",
		},
	}
}

// isMessageTypeEnabled checks if a message type is enabled in config
func isMessageTypeEnabled(config ProactiveConfig, msgType string) bool {
	for _, t := range config.EnabledMessageTypes {
		if t == msgType {
			return true
		}
	}
	return false
}
