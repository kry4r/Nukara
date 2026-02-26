package agent

// NanobotEvent represents an event from the nanobot extend-chat channel.
type NanobotEvent struct {
	Type           string        `json:"type"`
	ConversationID string        `json:"conversation_id,omitempty"`
	ClientMsgID    string        `json:"client_msg_id,omitempty"`
	ServerMsgID    string        `json:"server_msg_id,omitempty"`
	MsgID          string        `json:"msg_id,omitempty"`
	ReplyGroupID   string        `json:"reply_group_id,omitempty"`
	Sequence       int           `json:"sequence,omitempty"`
	Content        *EventContent `json:"content,omitempty"`
	IsTyping       *bool         `json:"is_typing,omitempty"`
	Count          int           `json:"count,omitempty"`
	Timestamp      int64         `json:"timestamp,omitempty"`
	Message        string        `json:"message,omitempty"`
}

// EventContent is the content payload in a nanobot event.
type EventContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Event type constants.
const (
	EventAck             = "ack"
	EventTyping          = "typing"
	EventBotWaiting      = "bot_waiting"
	EventMultiReplyStart = "multi_reply_start"
	EventMessage         = "message"
	EventMultiReplyEnd   = "multi_reply_end"
	EventProactive       = "proactiveMessage"
	EventPong            = "pong"
	EventError           = "error"
)

// ClientMsg is a message sent from Go to nanobot via WebSocket.
type ClientMsg struct {
	Type           string         `json:"type"`
	ConversationID string         `json:"conversation_id,omitempty"`
	RobotID        string         `json:"robot_id,omitempty"`
	ClientMsgID    string         `json:"client_msg_id,omitempty"`
	Content        *EventContent  `json:"content,omitempty"`
	SystemContext  map[string]any `json:"system_context,omitempty"`
}
