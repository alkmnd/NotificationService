package service

import (
	"encoding/json"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
	"log"
	"time"

	"github.com/google/uuid"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

type Client struct {
	// The actual websocket connection.
	ID       uuid.UUID
	conn     *websocket.Conn
	wsServer *WsServer
	send     chan []byte
	apiKey   string
}

func newClient(id uuid.UUID, apiKey string, conn *websocket.Conn, wsServer *WsServer) *Client {
	return &Client{
		ID:       id,
		conn:     conn,
		wsServer: wsServer,
		send:     make(chan []byte, 256),
		apiKey:   apiKey,
	}

}

func (client *Client) disconnect() {
	client.wsServer.unregister <- client
}

type tokenClaims struct {
	jwt.StandardClaims
	UserId uuid.UUID `json:"user_id"`
	Role   string    `json:"access"`
}

const (
	signingKey = "qrkjk#4#%35FSFJlja#4353KSFjH"
)

func parseToken(accessToken string) (id uuid.UUID, err error) {
	token, err := jwt.ParseWithClaims(accessToken, &tokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}

		return []byte(signingKey), nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(*tokenClaims)
	if !ok {
		return uuid.Nil, errors.New("token claims are not of type *tokenClaims")
	}

	return claims.UserId, nil

}

// ServeWs handles websocket requests from Clients requests.
func ServeWs(wsServer *WsServer, w http.ResponseWriter, r *http.Request) {
	queryApiKey, _ := r.URL.Query()["api_key"]
	var apiKey string
	if queryApiKey != nil {
		apiKey = queryApiKey[0]
	}

	token, _ := r.URL.Query()["token"]

	var id uuid.UUID
	if token != nil && token[0] != "" {
		id, _ = parseToken(token[0])
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	client := newClient(id, apiKey, conn, wsServer)

	go client.writePump()
	go client.readPump()

	wsServer.register <- client
}

const (
	// Max wait time when writing message to peer
	writeWait = 10 * time.Second

	// Max time till next pong from peer
	pongWait = 10 * time.Second

	// Send ping interval, must be less than pong wait time
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 10000
)

func (client *Client) readPump() {
	defer func() {
		client.disconnect()
	}()

	client.conn.SetReadLimit(maxMessageSize)
	client.conn.SetReadDeadline(time.Time{})
	client.conn.SetPongHandler(func(string) error { client.conn.SetReadDeadline(time.Time{}); return nil })

	// Start endless read loop, waiting for messages from client
	for {
		_, jsonMessage, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected close error: %v", err)
			}
			break
		}

		client.handleNewMessage(jsonMessage)
	}
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

func (client *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()
	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The WsServer closed the channel.
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Attach queued chat messages to the current websocket message.
			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (client *Client) handleNewMessage(jsonMessage []byte) {
	var message Message
	if err := json.Unmarshal(jsonMessage, &message); err != nil {
		log.Printf("handleNewMessage error on unmarshal JSON message %s", err)
		return
	}

	receiver := client.wsServer.findClientByID(message.UserId)
	if receiver == nil {
		return
	}

	if client.apiKey != client.wsServer.apiKey {
		return
	}

	receiver.send <- []byte("Notification")
}
