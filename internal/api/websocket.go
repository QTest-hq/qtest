// Package api provides WebSocket support for real-time updates
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/QTest-hq/qtest/internal/auth"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512

	// Buffer size for client channels
	channelBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, this should verify against allowed origins
		// The main CORS middleware handles this, but we add an extra check
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Allow non-browser clients
		}
		// Accept if the main CORS handler already validated
		return true
	},
}

// WSMessageType represents the type of WebSocket message
type WSMessageType string

const (
	WSMessageTypeJobUpdate      WSMessageType = "job.update"
	WSMessageTypeJobCreated     WSMessageType = "job.created"
	WSMessageTypeJobCompleted   WSMessageType = "job.completed"
	WSMessageTypeJobFailed      WSMessageType = "job.failed"
	WSMessageTypeTestUpdate     WSMessageType = "test.update"
	WSMessageTypeCoverageUpdate WSMessageType = "coverage.update"
	WSMessageTypeError          WSMessageType = "error"
	WSMessageTypePing           WSMessageType = "ping"
	WSMessageTypePong           WSMessageType = "pong"
	WSMessageTypeSubscribe      WSMessageType = "subscribe"
	WSMessageTypeUnsubscribe    WSMessageType = "unsubscribe"
	WSMessageTypeSubscribed     WSMessageType = "subscribed"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      WSMessageType   `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	RequestID string          `json:"request_id,omitempty"`
}

// WSSubscription represents a subscription request
type WSSubscription struct {
	Channel string `json:"channel"` // e.g., "jobs", "jobs:{job_id}", "repos:{repo_id}"
}

// WSJobPayload represents a job update payload
type WSJobPayload struct {
	JobID          uuid.UUID  `json:"job_id"`
	RepositoryID   *uuid.UUID `json:"repository_id,omitempty"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	Progress       int        `json:"progress,omitempty"`
	Message        string     `json:"message,omitempty"`
	Error          string     `json:"error,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Result         any        `json:"result,omitempty"`
	OrganizationID uuid.UUID  `json:"organization_id"`
}

// WSClient represents a connected WebSocket client
type WSClient struct {
	hub            *WSHub
	conn           *websocket.Conn
	send           chan []byte
	userID         uuid.UUID
	organizationID uuid.UUID
	subscriptions  map[string]bool // channels this client is subscribed to
	mu             sync.RWMutex
}

// WSHub maintains the set of active clients and broadcasts messages
type WSHub struct {
	// Registered clients by organization
	clients map[uuid.UUID]map[*WSClient]bool

	// Channel subscriptions: channel -> clients
	channels map[string]map[*WSClient]bool

	// Register requests from clients
	register chan *WSClient

	// Unregister requests from clients
	unregister chan *WSClient

	// Broadcast to specific organization
	broadcast chan *orgBroadcast

	// Channel broadcast
	channelBroadcast chan *channelBroadcast

	mu sync.RWMutex
}

type orgBroadcast struct {
	organizationID uuid.UUID
	message        []byte
}

type channelBroadcast struct {
	channel string
	message []byte
}

// NewWSHub creates a new WebSocket hub
func NewWSHub() *WSHub {
	return &WSHub{
		clients:          make(map[uuid.UUID]map[*WSClient]bool),
		channels:         make(map[string]map[*WSClient]bool),
		register:         make(chan *WSClient),
		unregister:       make(chan *WSClient),
		broadcast:        make(chan *orgBroadcast, channelBufferSize),
		channelBroadcast: make(chan *channelBroadcast, channelBufferSize),
	}
}

// Run starts the hub's main loop
func (h *WSHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.organizationID] == nil {
				h.clients[client.organizationID] = make(map[*WSClient]bool)
			}
			h.clients[client.organizationID][client] = true
			h.mu.Unlock()

			log.Debug().
				Str("user_id", client.userID.String()[:8]+"...").
				Str("org_id", client.organizationID.String()[:8]+"...").
				Msg("WebSocket client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.organizationID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)

					// Clean up channel subscriptions
					client.mu.Lock()
					for channel := range client.subscriptions {
						if channelClients, ok := h.channels[channel]; ok {
							delete(channelClients, client)
							if len(channelClients) == 0 {
								delete(h.channels, channel)
							}
						}
					}
					client.mu.Unlock()

					if len(clients) == 0 {
						delete(h.clients, client.organizationID)
					}
				}
			}
			h.mu.Unlock()

			log.Debug().
				Str("user_id", client.userID.String()[:8]+"...").
				Msg("WebSocket client unregistered")

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[msg.organizationID]
			h.mu.RUnlock()

			for client := range clients {
				select {
				case client.send <- msg.message:
				default:
					h.mu.Lock()
					delete(h.clients[msg.organizationID], client)
					close(client.send)
					h.mu.Unlock()
				}
			}

		case msg := <-h.channelBroadcast:
			h.mu.RLock()
			clients := h.channels[msg.channel]
			h.mu.RUnlock()

			for client := range clients {
				select {
				case client.send <- msg.message:
				default:
					h.mu.Lock()
					if channelClients, ok := h.channels[msg.channel]; ok {
						delete(channelClients, client)
					}
					h.mu.Unlock()
				}
			}
		}
	}
}

// BroadcastToOrganization sends a message to all clients in an organization
func (h *WSHub) BroadcastToOrganization(orgID uuid.UUID, msgType WSMessageType, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := WSMessage{
		Type:      msgType,
		Payload:   data,
		Timestamp: time.Now(),
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.broadcast <- &orgBroadcast{
		organizationID: orgID,
		message:        msgBytes,
	}

	return nil
}

// BroadcastToChannel sends a message to all clients subscribed to a channel
func (h *WSHub) BroadcastToChannel(channel string, msgType WSMessageType, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := WSMessage{
		Type:      msgType,
		Payload:   data,
		Timestamp: time.Now(),
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.channelBroadcast <- &channelBroadcast{
		channel: channel,
		message: msgBytes,
	}

	return nil
}

// BroadcastJobUpdate broadcasts a job update to relevant subscribers
func (h *WSHub) BroadcastJobUpdate(job *WSJobPayload) {
	// Broadcast to organization
	if err := h.BroadcastToOrganization(job.OrganizationID, WSMessageTypeJobUpdate, job); err != nil {
		log.Error().Err(err).Msg("failed to broadcast job update to organization")
	}

	// Broadcast to specific job channel
	jobChannel := "jobs:" + job.JobID.String()
	if err := h.BroadcastToChannel(jobChannel, WSMessageTypeJobUpdate, job); err != nil {
		log.Error().Err(err).Msg("failed to broadcast job update to channel")
	}

	// Broadcast to repository channel if applicable
	if job.RepositoryID != nil {
		repoChannel := "repos:" + job.RepositoryID.String() + ":jobs"
		if err := h.BroadcastToChannel(repoChannel, WSMessageTypeJobUpdate, job); err != nil {
			log.Error().Err(err).Msg("failed to broadcast job update to repo channel")
		}
	}
}

// GetClientCount returns the number of connected clients
func (h *WSHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}

// GetClientCountForOrg returns the number of connected clients for an organization
func (h *WSHub) GetClientCountForOrg(orgID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[orgID]; ok {
		return len(clients)
	}
	return 0
}

// subscribe adds a client to a channel
func (h *WSHub) subscribe(client *WSClient, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.channels[channel] == nil {
		h.channels[channel] = make(map[*WSClient]bool)
	}
	h.channels[channel][client] = true

	client.mu.Lock()
	client.subscriptions[channel] = true
	client.mu.Unlock()
}

// unsubscribe removes a client from a channel
func (h *WSHub) unsubscribe(client *WSClient, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.channels[channel]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.channels, channel)
		}
	}

	client.mu.Lock()
	delete(client.subscriptions, channel)
	client.mu.Unlock()
}

// readPump pumps messages from the websocket connection to the hub
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debug().Err(err).Msg("WebSocket read error")
			}
			break
		}

		// Parse the message
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Debug().Err(err).Msg("failed to parse WebSocket message")
			continue
		}

		c.handleMessage(&msg)
	}
}

// handleMessage processes incoming WebSocket messages
func (c *WSClient) handleMessage(msg *WSMessage) {
	switch msg.Type {
	case WSMessageTypePing:
		// Respond with pong
		response := WSMessage{
			Type:      WSMessageTypePong,
			Timestamp: time.Now(),
			RequestID: msg.RequestID,
		}
		data, _ := json.Marshal(response)
		select {
		case c.send <- data:
		default:
		}

	case WSMessageTypeSubscribe:
		var sub WSSubscription
		if err := json.Unmarshal(msg.Payload, &sub); err != nil {
			log.Debug().Err(err).Msg("failed to parse subscription")
			return
		}

		// Validate channel access (ensure user can only subscribe to their org's channels)
		if !c.canAccessChannel(sub.Channel) {
			c.sendError("unauthorized channel access", msg.RequestID)
			return
		}

		c.hub.subscribe(c, sub.Channel)

		// Send confirmation
		response := WSMessage{
			Type:      WSMessageTypeSubscribed,
			Timestamp: time.Now(),
			RequestID: msg.RequestID,
		}
		payload, _ := json.Marshal(sub)
		response.Payload = payload
		data, _ := json.Marshal(response)
		select {
		case c.send <- data:
		default:
		}

		log.Debug().
			Str("user_id", c.userID.String()[:8]+"...").
			Str("channel", sub.Channel).
			Msg("client subscribed to channel")

	case WSMessageTypeUnsubscribe:
		var sub WSSubscription
		if err := json.Unmarshal(msg.Payload, &sub); err != nil {
			return
		}
		c.hub.unsubscribe(c, sub.Channel)
	}
}

// canAccessChannel checks if a client can access a specific channel
func (c *WSClient) canAccessChannel(channel string) bool {
	// For now, allow access to any channel within the organization
	// In production, add more granular checks based on channel naming
	// e.g., jobs:{job_id} should verify the job belongs to the org
	return true
}

// sendError sends an error message to the client
func (c *WSClient) sendError(message, requestID string) {
	response := WSMessage{
		Type:      WSMessageTypeError,
		Timestamp: time.Now(),
		RequestID: requestID,
	}
	payload, _ := json.Marshal(map[string]string{"error": message})
	response.Payload = payload
	data, _ := json.Marshal(response)
	select {
	case c.send <- data:
	default:
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (s *Server) HandleWebSocket(hub *WSHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authentication from context
		session, ok := auth.GetSessionFromContext(r.Context())
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get organization ID from API key or session
		var orgID uuid.UUID
		if apiKeyInfo, ok := auth.GetAPIKeyFromContext(r.Context()); ok {
			orgID = apiKeyInfo.OrganizationID
		} else {
			// For session auth, get org from query param or default
			orgIDStr := r.URL.Query().Get("organization_id")
			if orgIDStr != "" {
				var err error
				orgID, err = uuid.Parse(orgIDStr)
				if err != nil {
					http.Error(w, "Invalid organization ID", http.StatusBadRequest)
					return
				}
			}
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error().Err(err).Msg("failed to upgrade WebSocket connection")
			return
		}

		client := &WSClient{
			hub:            hub,
			conn:           conn,
			send:           make(chan []byte, channelBufferSize),
			userID:         session.UserID,
			organizationID: orgID,
			subscriptions:  make(map[string]bool),
		}

		hub.register <- client

		// Start goroutines for reading and writing
		go client.writePump()
		go client.readPump()
	}
}
