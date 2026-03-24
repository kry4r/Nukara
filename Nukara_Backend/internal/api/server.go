package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	stdmail "net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/llm"
	"nukara/backend/internal/agentx/memorybuffer"
	"nukara/backend/internal/agentx/memorygraph"
	agentprovider "nukara/backend/internal/agentx/provider"
	"nukara/backend/internal/agentx/subtasks"
	"nukara/backend/internal/apns"
	internalmail "nukara/backend/internal/mail"
	"nukara/backend/internal/store"
)

const defaultEmailSendCooldown = 60 * time.Second

type wsChatRuntime interface {
	StreamTurn(ctx context.Context, req agentx.TurnRequest) (<-chan agentx.StreamDelta, <-chan agentx.FinalTurn, error)
}

type routeStore interface {
	ListProviders() ([]store.Provider, error)
	GetUserProviderSetting(userID string) (providerID, model string, ok bool)
	GetBotProviderOverride(userID, botID string) (providerID, model string, ok bool)
	GetSystemSetting(key string) (value string, ok bool)
}

type temporalMemoryRecall interface {
	Recall(ctx context.Context, in memorygraph.RecallInput) (memorygraph.RecallResult, error)
}

type memoryTool interface {
	Save(ctx context.Context, item store.MemoryItem) (store.MemoryItem, error)
}

type verificationEmailSender interface {
	SendVerificationCode(ctx context.Context, to, code string, ttl time.Duration) error
}

type Server struct {
	store          store.DataStore
	agent          *agent.Agent
	runtime        wsChatRuntime
	memoryTool     memoryTool
	temporalRecall temporalMemoryRecall
	wsQueue        *wsConversationQueue
	subtasks       interface {
		Run(ctx context.Context, in subtasks.Input) (subtasks.Result, error)
	}
	apns              *apns.Client
	wsHub             *wsHub
	emailSender       verificationEmailSender
	emailSendMu       sync.Mutex
	emailSendCooldown time.Duration
	tokenKey          []byte
	tokenTTL          time.Duration
	memoryBuffer      *memorybuffer.Service
	topicDetector     memorybuffer.TopicDetector
}

func NewServer(st store.DataStore, agentClient *agent.Agent, apnsClient *apns.Client, tokenSecret string, redisAddr string) *Server {
	if tokenSecret == "" {
		tokenSecret = "nukara-dev-secret"
	}
	s := &Server{
		store:             st,
		agent:             agentClient,
		apns:              apnsClient,
		wsHub:             newWSHub(st, redisAddr),
		emailSender:       internalmail.NewSMTPSender(st),
		emailSendCooldown: defaultEmailSendCooldown,
		tokenKey:          []byte(tokenSecret),
		tokenTTL:          30 * 24 * time.Hour,
		temporalRecall:    memorygraph.NewService(memorygraph.ServiceDeps{Store: st}),
		memoryBuffer:      memorybuffer.NewService(memorybuffer.Options{}),
		topicDetector:     memorybuffer.NoOpTopicDetector{},
	}
	if agentClient != nil {
		deps := agentx.RuntimeDeps{
			ProviderClient: llm.NewLegacyAgentClient(agentClient),
		}
		if rs, ok := st.(routeStore); ok {
			deps.RouteResolver = agentprovider.NewRouter(rs)
		}
		s.runtime = agentx.NewRuntime(deps)
	}
	s.initSubtasks()
	s.wsQueue = newWSConversationQueue(s)
	return s
}

func (s *Server) initSubtasks() {
	s.subtasks = subtasks.NewRunner(subtasks.RunnerDeps{
		Store:       s.store,
		MemoryTool:  s.memoryTool,
		MemoryGraph: memorygraph.NewService(memorygraph.ServiceDeps{Store: s.store}),
		MemoryExtractor: func(ctx context.Context, in subtasks.Input) (string, error) {
			var prompt string
			if strings.TrimSpace(in.AggregatedText) != "" {
				// Multi-turn aggregated flush: use the dedicated aggregated prompt.
				prompt = buildMemoryExtractPromptAggregated(
					in.AggregatedText,
					s.buildMemoryCandidateContext(in.UserID, in.BotID, in.UserText, in.BotText),
				)
			} else {
				prompt = buildMemoryExtractPrompt(in.UserText, in.BotText, s.buildMemoryCandidateContext(in.UserID, in.BotID, in.UserText, in.BotText))
			}
			return s.runSubtaskPrompt(ctx, in, prompt, `{"items":[]}`)
		},
		CompactUpdater: func(ctx context.Context, in subtasks.Input) (string, error) {
			prompt := buildCompactPrompt(in.UserText, in.BotText)
			return s.runSubtaskPrompt(ctx, in, prompt, `{"summary":"","facts":[]}`)
		},
		PersonaIterator: func(ctx context.Context, in subtasks.Input) (string, error) {
			prompt := buildPersonaIteratePrompt(in.UserText, in.BotText)
			return s.runSubtaskPrompt(ctx, in, prompt, `{"identity_adds":[],"personality_adds":[],"expression_style_adds":[],"life_context_adds":[],"taboos_and_preferences_adds":[]}`)
		},
		SelfCognitionUpdater: func(ctx context.Context, in subtasks.Input, bot store.Bot, changes []store.PersonaChangeEvent) (store.Bot, error) {
			return s.refreshBotSelfCognition(ctx, in, bot, changes)
		},
	})
}

func (s *Server) SetChatRuntime(runtime wsChatRuntime) {
	s.runtime = runtime
}

func (s *Server) SetTemporalMemoryRecall(recall temporalMemoryRecall) {
	s.temporalRecall = recall
}

func (s *Server) Handler() http.Handler {
	return s.HandlerFor("gateway")
}

func (s *Server) HandlerFor(role string) http.Handler {
	mux := http.NewServeMux()

	if role == "gateway" || role == "account" {
		mux.HandleFunc("/api/v1/auth/email/send", s.wrap(s.handleEmailSend))
		mux.HandleFunc("/api/v1/auth/login", s.wrap(s.handleLogin))
		mux.HandleFunc("/api/v1/auth/register", s.wrap(s.handleRegister))
	}
	if role == "gateway" {
		mux.HandleFunc("/api/v1/gateway/test/session", s.wrap(s.handleGatewayTestSession))
	}

	if role == "gateway" || role == "bot" {
		mux.HandleFunc("/api/v1/bots", s.wrap(s.handleBots))
		mux.HandleFunc("/api/v1/bots/", s.wrap(s.handleBotByID))
	}

	if role == "gateway" || role == "conversation" {
		mux.HandleFunc("/api/v1/conversations", s.wrap(s.handleConversations))
		mux.HandleFunc("/api/v1/conversations/", s.wrap(s.handleConversationByID))
		mux.HandleFunc("/api/v1/gateway/test/chat", s.wrap(s.handleGatewayTestChat))
		mux.HandleFunc("/api/v1/gateway/test/chat/stream", s.wrap(s.handleGatewayTestChatStream))
		mux.HandleFunc("/ws/chat", s.wrap(s.handleWSChat))
	}

	if role == "gateway" || role == "account" {
		mux.HandleFunc("/api/v1/users/status", s.wrap(s.handleUserStatus))
	}

	if role == "gateway" || role == "proactive" {
		mux.HandleFunc("/api/v1/users/device-token", s.wrap(s.handleDeviceToken))
		mux.HandleFunc("/api/v1/users/notification-settings", s.wrap(s.handleNotificationSettings))
		mux.HandleFunc("/api/v1/proactive/logs", s.wrap(s.handleProactiveLogs))
		mux.HandleFunc("/api/v1/gateway/test/proactive", s.wrap(s.handleGatewayTestProactive))
	}

	mux.HandleFunc("/api/v1/gateway/health", s.wrap(s.handleHealth))
	mux.HandleFunc("/api/v1/gateway/metrics", s.wrap(s.handleMetrics))

	return mux
}

func (s *Server) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.store.IncrementRequests()
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func (s *Server) handleEmailSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Email   string `json:"email"`
		Purpose string `json:"purpose"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	if err := validateEmail(email); err != nil || (req.Purpose != "login" && req.Purpose != "register") {
		badRequest(w, errors.New("invalid email or purpose"))
		return
	}
	if s.emailSender == nil {
		badRequest(w, errors.New("smtp not configured"))
		return
	}
	ttl := 15 * time.Minute
	if cfg, err := internalmail.LoadSMTPConfig(s.store); err == nil && cfg.CodeTTL > 0 {
		ttl = cfg.CodeTTL
	}

	s.emailSendMu.Lock()
	defer s.emailSendMu.Unlock()
	if existing, ok := s.store.GetLatestEmailCode(email, req.Purpose); ok {
		now := time.Now().UTC()
		if !existing.ExpiresAt.IsZero() && existing.ExpiresAt.After(now) && !existing.CreatedAt.IsZero() && now.Sub(existing.CreatedAt.UTC()) < s.emailSendCooldown {
			respondJSON(w, http.StatusOK, map[string]any{"success": true, "message": "验证码已发送到邮箱"})
			return
		}
	}

	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	if err := s.emailSender.SendVerificationCode(r.Context(), email, code, ttl); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "smtp not configured") && !strings.Contains(strings.ToLower(err.Error()), "invalid smtp") {
			respondJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		// dev mode: SMTP not configured, log code to console instead of sending email
		log.Printf("[DEV] email=%s purpose=%s code=%s (smtp not configured, use this code)", email, req.Purpose, code)
	} else {
		log.Printf("[EMAIL] email=%s purpose=%s code=%s", email, req.Purpose, code)
	}
	s.store.SaveEmailCode(email, req.Purpose, code, ttl)
	respondJSON(w, http.StatusOK, map[string]any{"success": true, "message": "验证码已发送到邮箱"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		Email     string `json:"email"`
		EmailCode string `json:"email_code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	code := strings.TrimSpace(req.EmailCode)
	if err := validateEmail(email); err != nil {
		badRequest(w, err)
		return
	}
	if code == "" {
		badRequest(w, errors.New("email_code required"))
		return
	}
	if !s.store.ValidateEmailCode(email, "login", code) && !s.store.ValidateEmailCode(email, "register", code) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "验证码错误或过期"})
		return
	}

	user, ok := s.store.FindUserByEmail(email)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "邮箱未注册"})
		return
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": "token issue failed"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"access_token":  token,
		"refresh_token": "",
		"user": map[string]any{
			"id":       user.ID,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
		},
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		Email     string `json:"email"`
		EmailCode string `json:"email_code"`
		Nickname  string `json:"nickname"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}

	email := strings.TrimSpace(req.Email)
	code := strings.TrimSpace(req.EmailCode)
	if email == "" || strings.TrimSpace(req.Nickname) == "" {
		badRequest(w, errors.New("email and nickname required"))
		return
	}
	if err := validateEmail(email); err != nil {
		badRequest(w, err)
		return
	}
	if code == "" {
		badRequest(w, errors.New("email_code required"))
		return
	}

	if !s.store.ValidateEmailCode(email, "register", code) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "验证码错误或过期"})
		return
	}

	user, err := s.store.CreateUser(email, strings.TrimSpace(req.Nickname))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": "token issue failed"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"access_token":  token,
		"refresh_token": "",
		"user": map[string]any{
			"id":       user.ID,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
		},
	})
}

func (s *Server) handleGatewayTestSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Nickname string `json:"nickname"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}

	email := strings.TrimSpace(req.Email)
	if err := validateEmail(email); err != nil {
		badRequest(w, err)
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = "smoke_user"
	}

	user, ok := s.store.FindUserByEmail(email)
	created := false
	if !ok {
		var err error
		user, err = s.store.CreateUser(email, nickname)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		created = true
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": "token issue failed"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"access_token":  token,
		"refresh_token": "",
		"created":       created,
		"user": map[string]any{
			"id":       user.ID,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
		},
	})
}

func (s *Server) handleBots(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		bots := s.store.ListBots(userID)
		respondJSON(w, http.StatusOK, bots)
	case http.MethodPost:
		var req struct {
			Name                 string   `json:"name"`
			Description          string   `json:"description"`
			Summary              string   `json:"summary"`
			Relationship         string   `json:"relationship"`
			Role                 string   `json:"role"`
			SelfCognition        []string `json:"self_cognition"`
			SpeakingStyle        string   `json:"speaking_style"`
			Background           string   `json:"background"`
			Traits               []string `json:"traits"`
			Identity             string   `json:"identity"`
			Personality          []string `json:"personality"`
			ExpressionStyle      string   `json:"expression_style"`
			LifeContext          string   `json:"life_context"`
			TaboosAndPreferences string   `json:"taboos_and_preferences"`
			Gender               string   `json:"gender"`
			AvatarBase64         string   `json:"avatar_base64"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			badRequest(w, errors.New("name required"))
			return
		}
		identity := strings.TrimSpace(req.Identity)
		if identity == "" {
			identity = firstNonEmpty(strings.TrimSpace(req.Summary), strings.TrimSpace(req.Description), strings.TrimSpace(req.Relationship))
		}
		personality := append([]string(nil), req.Personality...)
		if len(personality) == 0 {
			personality = append([]string(nil), req.Traits...)
		}
		expressionStyle := firstNonEmpty(strings.TrimSpace(req.ExpressionStyle), strings.TrimSpace(req.SpeakingStyle))
		lifeContext := firstNonEmpty(strings.TrimSpace(req.LifeContext), strings.TrimSpace(req.Background), strings.TrimSpace(req.Role))
		taboos := strings.TrimSpace(req.TaboosAndPreferences)
		if req.Gender == "" {
			req.Gender = "unknown"
		}

		created := s.store.CreateBot(userID, store.Bot{
			Name:                 strings.TrimSpace(req.Name),
			Identity:             identity,
			Personality:          personality,
			ExpressionStyle:      expressionStyle,
			LifeContext:          lifeContext,
			TaboosAndPreferences: taboos,
			Summary:              strings.TrimSpace(req.Summary),
			Relationship:         strings.TrimSpace(req.Relationship),
			Role:                 strings.TrimSpace(req.Role),
			SelfCognition:        req.SelfCognition,
			SpeakingStyle:        strings.TrimSpace(req.SpeakingStyle),
			Background:           strings.TrimSpace(req.Background),
			Traits:               req.Traits,
			Gender:               req.Gender,
			AvatarBase64:         req.AvatarBase64,
			ChatBackgroundStyle:  "lightPaper",
		})

		respondJSON(w, http.StatusCreated, created)

		// Generate starter message async so bot creation returns immediately.
		go func(uid string, bot store.Bot) {
			conv, found := s.store.FindConversationByBot(uid, bot.ID)
			if !found {
				return
			}
			starterText := s.generateStarterMessage(uid, bot, conv.ID)
			msg, _ := s.store.SaveMessage(uid, store.Message{
				ConversationID: conv.ID,
				SenderType:     "bot",
				ContentType:    "text",
				Content:        store.MessageContent{Type: "text", Text: starterText},
				IsProactive:    true,
			})
			s.wsHub.publishToUser(uid, wsProactiveEvent(msg))
		}(userID, created)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleBotByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/bots/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot id missing"})
		return
	}
	botID := parts[0]

	// Sub-resource: /bots/{botID}/directives[/{id}]
	if len(parts) >= 2 {
		switch parts[1] {
		case "directives":
			s.handleBotDirectives(w, r, userID, botID, parts[2:])
			return
		case "profile":
			s.handleBotProfile(w, r, userID, botID)
			return
		case "impression":
			s.handleBotImpression(w, r, userID, botID)
			return
		case "iterate":
			s.handleBotIterate(w, r, userID, botID)
			return
		case "memories":
			s.handleBotMemories(w, r, userID, botID, parts[2:])
			return
		case "persona-changes":
			if len(parts) >= 4 && (parts[3] == "accept" || parts[3] == "reject") {
				s.handleBotPersonaChangeAction(w, r, userID, botID, parts[2], parts[3])
				return
			}
		}
	}

	switch r.Method {
	case http.MethodGet:
		bot, found := s.store.GetBot(userID, botID)
		if !found {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
			return
		}
		respondJSON(w, http.StatusOK, bot)
	case http.MethodPut:
		var req struct {
			Name                 string   `json:"name"`
			Description          string   `json:"description"`
			Summary              string   `json:"summary"`
			Relationship         string   `json:"relationship"`
			Role                 string   `json:"role"`
			SelfCognition        []string `json:"self_cognition"`
			SpeakingStyle        string   `json:"speaking_style"`
			Background           string   `json:"background"`
			Traits               []string `json:"traits"`
			Identity             string   `json:"identity"`
			Personality          []string `json:"personality"`
			ExpressionStyle      string   `json:"expression_style"`
			LifeContext          string   `json:"life_context"`
			TaboosAndPreferences string   `json:"taboos_and_preferences"`
			Gender               string   `json:"gender"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		identity := strings.TrimSpace(req.Identity)
		if identity == "" {
			identity = firstNonEmpty(strings.TrimSpace(req.Summary), strings.TrimSpace(req.Description), strings.TrimSpace(req.Relationship))
		}
		personality := append([]string(nil), req.Personality...)
		if len(personality) == 0 {
			personality = append([]string(nil), req.Traits...)
		}
		expressionStyle := firstNonEmpty(strings.TrimSpace(req.ExpressionStyle), strings.TrimSpace(req.SpeakingStyle))
		lifeContext := firstNonEmpty(strings.TrimSpace(req.LifeContext), strings.TrimSpace(req.Background), strings.TrimSpace(req.Role))
		taboos := strings.TrimSpace(req.TaboosAndPreferences)
		updated, found := s.store.UpdateBot(userID, botID, store.Bot{
			Name:                 req.Name,
			Identity:             identity,
			Personality:          personality,
			ExpressionStyle:      expressionStyle,
			LifeContext:          lifeContext,
			TaboosAndPreferences: taboos,
			Summary:              strings.TrimSpace(req.Summary),
			Relationship:         strings.TrimSpace(req.Relationship),
			Role:                 strings.TrimSpace(req.Role),
			SelfCognition:        req.SelfCognition,
			SpeakingStyle:        req.SpeakingStyle,
			Background:           req.Background,
			Traits:               req.Traits,
			Gender:               req.Gender,
		})
		if !found {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
			return
		}
		respondJSON(w, http.StatusOK, updated)
	case http.MethodPatch:
		var req struct {
			SpeakingStyleAdds []string `json:"speaking_style_adds"`
			BackgroundAdds    []string `json:"background_adds"`
			TraitAdds         []string `json:"trait_adds"`
			Gender            string   `json:"gender"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}

		var genderPtr *string
		if strings.TrimSpace(req.Gender) != "" {
			gender := strings.TrimSpace(req.Gender)
			genderPtr = &gender
		}

		bot, found := s.store.AppendBotPersona(userID, botID, req.SpeakingStyleAdds, req.BackgroundAdds, req.TraitAdds, genderPtr)
		if !found {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
			return
		}
		respondJSON(w, http.StatusOK, bot)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleBotDirectives(w http.ResponseWriter, r *http.Request, userID, botID string, rest []string) {
	if _, found := s.store.GetBot(userID, botID); !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}
	switch {
	case r.Method == http.MethodGet && len(rest) == 0:
		respondJSON(w, http.StatusOK, s.store.ListDirectives(userID, botID, "active"))
	case r.Method == http.MethodDelete && len(rest) == 1 && rest[0] != "":
		if s.store.RevokeDirective(userID, botID, rest[0]) {
			respondJSON(w, http.StatusOK, map[string]any{"ok": true})
		} else {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "directive not found"})
		}
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	conversations := s.store.ListConversations(userID)
	out := make([]map[string]any, 0, len(conversations))
	for _, conv := range conversations {
		state, _ := s.store.GetBotState(userID, conv.BotID)
		out = append(out, map[string]any{
			"id":                   conv.ID,
			"bot_id":               conv.BotID,
			"bot_name":             conv.BotName,
			"bot_avatar":           conv.BotAvatar,
			"bot_avatar_base64":    conv.BotAvatarBase64,
			"bot_status_emoji":     fallback(state.StatusEmoji, "🙂"),
			"bot_status_text":      fallback(state.StatusText, "在线"),
			"last_message":         conv.LastMessage,
			"last_message_at":      conv.LastMessageAt.UTC().Format(time.RFC3339),
			"unread_count":         conv.UnreadCount,
			"is_proactive_message": conv.IsProactiveMessage,
		})
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleConversationByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "invalid path"})
		return
	}
	convID := parts[0]
	action := parts[1]

	switch action {
	case "messages":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		limit := 100
		if q := r.URL.Query().Get("limit"); q != "" {
			if parsed, err := strconv.Atoi(q); err == nil {
				if parsed > 0 && parsed <= 200 {
					limit = parsed
				}
			}
		}
		messages, found := s.store.ListMessages(userID, convID, limit)
		if !found {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
			return
		}
		respondJSON(w, http.StatusOK, messages)
	case "send":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req struct {
			ClientMessageID string `json:"client_msg_id"`
			Content         struct {
				Type        string   `json:"type"`
				Text        string   `json:"text"`
				ImageBase64 string   `json:"image_base64"`
				Latitude    *float64 `json:"latitude"`
				Longitude   *float64 `json:"longitude"`
				Name        string   `json:"name"`
			} `json:"content"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		result, err := s.processChatMessage(userID, chatMessageRequest{
			ConversationID:  convID,
			ClientMessageID: req.ClientMessageID,
			Content: store.MessageContent{
				Type:        req.Content.Type,
				Text:        req.Content.Text,
				ImageBase64: req.Content.ImageBase64,
				Latitude:    req.Content.Latitude,
				Longitude:   req.Content.Longitude,
				Name:        req.Content.Name,
			},
		})
		if err != nil {
			switch err {
			case errConversationNotFound:
				respondJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			case errBotNotFound:
				respondJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			case errInvalidContent:
				badRequest(w, err)
			default:
				respondJSON(w, http.StatusInternalServerError, map[string]any{"error": "chat process failed"})
			}
			return
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"ack": map[string]any{
				"client_msg_id": fallback(req.ClientMessageID, result.UserMessage.ID),
				"server_msg_id": result.UserMessage.ID,
				"timestamp":     result.UserMessage.CreatedAt.Unix(),
			},
			"bot_message": result.BotMessage,
			"bot_status_update": map[string]any{
				"conversation_id": result.Conversation.ID,
				"emoji":           result.StatusEmoji,
				"text":            result.StatusText,
				"status": map[string]any{
					"emoji": result.StatusEmoji,
					"text":  result.StatusText,
				},
			},
		})
	case "mark-read", "read":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		ok := s.store.MarkConversationRead(userID, convID)
		if !ok {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "unsupported conversation action"})
	}
}

func (s *Server) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		DeviceToken string `json:"device_token"`
		Platform    string `json:"platform"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.DeviceToken) == "" {
		badRequest(w, errors.New("device_token required"))
		return
	}
	if req.Platform == "" {
		req.Platform = "ios"
	}

	s.store.UpsertDeviceToken(userID, strings.TrimSpace(req.DeviceToken), req.Platform)
	respondJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleNotificationSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, s.store.GetNotificationSettings(userID))
	case http.MethodPut:
		var req struct {
			ProactiveEnabled         bool   `json:"proactive_enabled"`
			DNDStart                 string `json:"dnd_start"`
			DNDEnd                   string `json:"dnd_end"`
			ProactiveIntervalMinutes int    `json:"proactive_interval_minutes"`
			Frequency                string `json:"frequency"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		saved := s.store.UpdateNotificationSettings(userID, store.NotificationSettings{
			ProactiveEnabled:         req.ProactiveEnabled,
			DNDStart:                 strings.TrimSpace(req.DNDStart),
			DNDEnd:                   strings.TrimSpace(req.DNDEnd),
			ProactiveIntervalMinutes: req.ProactiveIntervalMinutes,
			Frequency:                strings.TrimSpace(req.Frequency),
		})
		respondJSON(w, http.StatusOK, saved)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleUserStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		st, found := s.store.GetUserStatus(userID)
		if !found {
			respondJSON(w, http.StatusOK, map[string]any{"user_id": userID, "emoji": "", "text": ""})
			return
		}
		respondJSON(w, http.StatusOK, st)
	case http.MethodPut:
		var req struct {
			Emoji string `json:"emoji"`
			Text  string `json:"text"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		s.store.SaveUserStatus(userID, req.Emoji, req.Text)
		s.wsHub.publishToUser(userID, map[string]any{
			"type":  "user_status_update",
			"emoji": req.Emoji,
			"text":  req.Text,
		})
		respondJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleProactiveLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	respondJSON(w, http.StatusOK, s.store.ListProactiveLogs(userID, limit))
}

func (s *Server) handleGatewayTestChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		BotID   string `json:"bot_id"`
		Message string `json:"message"`
		Debug   bool   `json:"debug"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}

	bot, found := s.store.GetBot(userID, req.BotID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	conv := s.store.EnsureConversation(userID, bot.ID, bot.Name, bot.Avatar, bot.AvatarBase64)
	start := time.Now()

	_, _ = s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "user",
		ContentType:    "text",
		Content:        store.MessageContent{Type: "text", Text: strings.TrimSpace(req.Message)},
	})

	providerConversationID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	sysCtx := agent.BuildSystemContext(bot, nil)
	reply, emotion, _, _, chatErr := s.runRuntimeChatTextWithProviderConversation(context.Background(), userID, bot.ID, conv.ID, providerConversationID, req.Message, sysCtx)
	if chatErr != nil {
		log.Printf("[server] runtime chat failed: %v", chatErr)
		reply = fmt.Sprintf("%s：我记住了你说的。要不要继续聊聊？", bot.Name)
		emotion = "gentle"
	}
	_, _ = s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "bot",
		ContentType:    "text",
		Content:        store.MessageContent{Type: "text", Text: reply},
		EmotionTag:     emotion,
	})

	latency := time.Since(start).Milliseconds()
	resp := map[string]any{
		"reply":          reply,
		"token_consumed": len(req.Message) + len(reply),
		"latency_ms":     latency,
	}
	if req.Debug {
		resp["debug"] = map[string]any{
			"model_used":          "agentx-runtime",
			"total_input_tokens":  len(req.Message),
			"total_output_tokens": len(reply),
		}
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGatewayTestChatStream(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		BotID   string `json:"bot_id"`
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}

	bot, found := s.store.GetBot(userID, req.BotID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	conv := s.store.EnsureConversation(userID, bot.ID, bot.Name, bot.Avatar, bot.AvatarBase64)
	_, _ = s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "user",
		ContentType:    "text",
		Content:        store.MessageContent{Type: "text", Text: strings.TrimSpace(req.Message)},
	})

	providerConversationID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	sysCtx := agent.BuildSystemContext(bot, nil)
	reply, emotion, _, _, chatErr := s.runRuntimeChatTextWithProviderConversation(context.Background(), userID, bot.ID, conv.ID, providerConversationID, req.Message, sysCtx)
	if chatErr != nil {
		log.Printf("[server] runtime chat(stream) failed: %v", chatErr)
		reply = fmt.Sprintf("%s：我记住了你说的。要不要继续聊聊？", bot.Name)
		emotion = "gentle"
	}
	_, _ = s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "bot",
		ContentType:    "text",
		Content:        store.MessageContent{Type: "text", Text: reply},
		EmotionTag:     emotion,
	})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	msgID := store.NewID()
	fmt.Fprintf(w, "event: start\ndata: {\"msg_id\":\"%s\"}\n\n", msgID)
	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	chunks := chunkString(reply, 12)
	for _, chunk := range chunks {
		fmt.Fprintf(w, "event: chunk\ndata: {\"text\":%q}\n\n", chunk)
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Fprintf(w, "event: done\ndata: {\"token_consumed\":%d,\"latency_ms\":%d}\n\n", len(req.Message)+len(reply), len(chunks)*50)
	if flusher != nil {
		flusher.Flush()
	}
}

func (s *Server) handleGatewayTestProactive(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		BotID          string `json:"bot_id"`
		ConversationID string `json:"conversation_id"`
		TriggerType    string `json:"trigger_type"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}

	bot, found := s.store.GetBot(userID, req.BotID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	settings := s.store.GetNotificationSettings(userID)
	if !settings.ProactiveEnabled {
		respondJSON(w, http.StatusOK, map[string]any{"should_send": false, "reason": "proactive_disabled"})
		return
	}
	localNow := InferLocaleContext(bot.LifeContext, time.Now().UTC()).LocalNow()
	if localNow.IsZero() {
		localNow = time.Now().UTC()
	}
	if isDND(settings, localNow) {
		respondJSON(w, http.StatusOK, map[string]any{"should_send": false, "reason": "dnd_active"})
		return
	}

	conv, found := s.store.GetConversation(userID, req.ConversationID)
	if !found {
		conv = s.store.EnsureConversation(userID, bot.ID, bot.Name, bot.Avatar, bot.AvatarBase64)
	}

	if req.TriggerType == "" {
		req.TriggerType = "manual"
	}

	localConversationID := conv.ID
	sysCtx := agent.BuildSystemContext(bot, nil)
	message, _, _, _, proactiveErr := s.runRuntimeProactive(context.Background(), userID, bot.ID, localConversationID, req.TriggerType, sysCtx)
	if proactiveErr != nil {
		log.Printf("[server] runtime proactive failed: %v", proactiveErr)
		message = fmt.Sprintf("%s：刚想到你了，最近怎么样？", bot.Name)
	}
	storedMessage, _ := s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "bot",
		ContentType:    "text",
		Content:        store.MessageContent{Type: "text", Text: message},
		IsProactive:    true,
		EmotionTag:     "gentle",
	})

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

	logEntry := s.store.AddProactiveLog(store.ProactiveLog{
		UserID:         userID,
		ConversationID: conv.ID,
		BotID:          bot.ID,
		TriggerType:    req.TriggerType,
		Message:        message,
		SentByWS:       sentByWS,
		SentByAPNs:     sentAPNs,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"should_send": true,
		"message": map[string]any{
			"id":              storedMessage.ID,
			"conversation_id": conv.ID,
			"content":         storedMessage.Content,
			"is_proactive":    true,
			"created_at":      storedMessage.CreatedAt.UTC().Format(time.RFC3339),
		},
		"sent_by_apns": sentAPNs,
		"log":          logEntry,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	respondJSON(w, http.StatusOK, s.store.SnapshotMetrics())
}

func (s *Server) authUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	var token string
	switch {
	case strings.HasPrefix(authHeader, "Bearer "):
		token = strings.TrimPrefix(authHeader, "Bearer ")
	case r.URL.Query().Get("token") != "":
		token = r.URL.Query().Get("token")
	default:
		respondJSON(w, http.StatusUnauthorized, map[string]any{"error": "missing authorization"})
		return "", false
	}
	userID, err := s.verifyToken(token)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid token"})
		return "", false
	}
	if _, exists := s.store.FindUserByID(userID); !exists {
		respondJSON(w, http.StatusUnauthorized, map[string]any{"error": "session invalidated"})
		return "", false
	}
	return userID, true
}

func (s *Server) issueToken(userID string) (string, error) {
	payload := map[string]any{
		"uid": userID,
		"exp": time.Now().Add(s.tokenTTL).Unix(),
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(rawPayload)
	sig := hmac.New(sha256.New, s.tokenKey)
	_, _ = sig.Write([]byte(payloadPart))
	signaturePart := base64.RawURLEncoding.EncodeToString(sig.Sum(nil))
	return payloadPart + "." + signaturePart, nil
}

func (s *Server) verifyToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", errors.New("invalid token format")
	}

	sig := hmac.New(sha256.New, s.tokenKey)
	_, _ = sig.Write([]byte(parts[0]))
	expected := sig.Sum(nil)

	received, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	if !hmac.Equal(expected, received) {
		return "", errors.New("signature mismatch")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var payload struct {
		UID string `json:"uid"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", err
	}
	if payload.UID == "" || time.Now().Unix() > payload.Exp {
		return "", errors.New("token expired")
	}
	return payload.UID, nil
}

func chunkString(text string, maxLen int) []string {
	if maxLen <= 0 || len(text) <= maxLen {
		return []string{text}
	}
	chunks := make([]string, 0, len(text)/maxLen+1)
	runes := []rune(text)
	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func validateEmail(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("email required")
	}
	if _, err := stdmail.ParseAddress(value); err != nil {
		return errors.New("invalid email")
	}
	return nil
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func badRequest(w http.ResponseWriter, err error) {
	respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	respondJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
}

func (s *Server) handleBotMemories(w http.ResponseWriter, r *http.Request, userID, botID string, rest []string) {
	if _, found := s.store.GetBot(userID, botID); !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	// DELETE /api/v1/bots/{botID}/memories/{memoryID}
	if r.Method == http.MethodDelete && len(rest) > 0 {
		memoryID := strings.TrimSpace(rest[0])
		if memoryID == "" {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": "memory id missing"})
			return
		}
		if err := s.store.DeleteMemoryNode(memoryID, userID, botID); err != nil {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "memory not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// GET /api/v1/bots/{botID}/memories
	if r.Method == http.MethodGet {
		nodes := s.store.ListMemoryNodes(userID, botID, store.TemporalMemoryNodeFilter{
			Status: "active",
			Limit:  100,
		})
		type memoryItem struct {
			ID             string `json:"id"`
			NodeType       string `json:"node_type"`
			Title          string `json:"title"`
			Summary        string `json:"summary"`
			OccurredAt     string `json:"occurred_at"`
			StabilityLabel string `json:"stability_label"`
		}
		items := make([]memoryItem, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, memoryItem{
				ID:             n.ID,
				NodeType:       n.NodeType,
				Title:          n.Title,
				Summary:        n.Summary,
				OccurredAt:     n.OccurredAt.UTC().Format(time.RFC3339),
				StabilityLabel: n.StabilityLabel,
			})
		}
		respondJSON(w, http.StatusOK, map[string]any{"memories": items})
		return
	}

	methodNotAllowed(w)
}
