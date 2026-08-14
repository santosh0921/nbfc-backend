// Package signaling implements a minimal WebRTC signaling relay for
// peer-to-peer video calls between an employee and the customer on a
// specific loan. The server never looks at (let alone stores) any call
// media — it only relays opaque signaling messages (SDP offers/answers,
// ICE candidates, and small call-control messages like "ringing" or
// "reject") between exactly the two parties in one loan's "room", the
// same job a signaling server plays in every WebRTC deployment. The
// actual audio/video stream, once connected, flows directly
// peer-to-peer and never touches this server at all.
package signaling

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// This endpoint is only ever called by the native customer/employee
	// apps (never a browser page), so there's no cross-site WebSocket
	// hijacking concern from relaxing the origin check the way there
	// would be for a browser-facing endpoint.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn *websocket.Conn
	room *room
	role string // "customer" | "employee"
	send chan []byte
}

type room struct {
	mu      sync.Mutex
	clients map[*client]bool
}

func (r *room) join(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = true
}

func (r *room) leave(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
}

// evictRole force-closes any existing connection(s) already in the room
// under the same role before a new one joins — rooms are keyed only by
// loanID with no per-call session identifier, so a connection left over
// from a previous call attempt (app backgrounded mid-call, a dropped
// network link the server hasn't noticed yet, a rapid double-tap on
// "Start Video Call") could otherwise still be sitting in r.clients and
// receive/interfere with a fresh call's signaling — e.g. a stray
// leftover employee-role connection getting a customer's call-request
// instead of (or in a race with) the real live one, which is exactly
// the kind of thing that would make the wrong party appear to ring, or
// the right party silently never ring at all. Closing the stale
// connection's socket triggers its own readPump's error path, which
// cleans it out of the room through the normal leave() flow.
func (r *room) evictRole(role string, except *client) {
	r.mu.Lock()
	var stale []*client
	for c := range r.clients {
		if c.role == role && c != except {
			stale = append(stale, c)
		}
	}
	r.mu.Unlock()
	for _, c := range stale {
		_ = c.conn.Close()
	}
}

// broadcastExcept relays a message to every OTHER client in the room —
// with at most 2 participants (customer, employee) this is always
// exactly "relay to the other party", but written generally in case a
// room ever needs an observer/supervisor third participant later.
func (r *room) broadcastExcept(sender *client, message []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		if c == sender {
			continue
		}
		select {
		case c.send <- message:
		default:
			// Slow/stuck client — drop rather than block the relay for
			// everyone else in the room.
		}
	}
}

var (
	roomsMu sync.Mutex
	rooms   = map[string]*room{}
)

func getOrCreateRoom(loanID string) *room {
	roomsMu.Lock()
	defer roomsMu.Unlock()
	r, ok := rooms[loanID]
	if !ok {
		r = &room{clients: map[*client]bool{}}
		rooms[loanID] = r
	}
	return r
}

func dropRoomIfEmpty(loanID string, r *room) {
	r.mu.Lock()
	empty := len(r.clients) == 0
	r.mu.Unlock()
	if !empty {
		return
	}
	roomsMu.Lock()
	defer roomsMu.Unlock()
	// Re-check under the rooms lock — another connection could have
	// joined between the unlock above and acquiring this lock.
	if current, ok := rooms[loanID]; ok && current == r {
		r.mu.Lock()
		stillEmpty := len(r.clients) == 0
		r.mu.Unlock()
		if stillEmpty {
			delete(rooms, loanID)
		}
	}
}

// CallSignalHandler handles GET /ws/call/:loanId — upgrades to a
// WebSocket and relays signaling messages between whichever two parties
// (identified by the same customer/employee auth already used for
// /notifications — see middleware.CustomerOrEmployeeAuthMiddleware)
// connect to the same loanId. Every message received from one side is
// forwarded verbatim to the other side; this handler does not need to
// understand (or trust) the message's contents beyond that.
func CallSignalHandler(c *gin.Context) {
	loanID := c.Param("loanId")
	if loanID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "loanId is required"})
		return
	}

	role := "customer"
	if empID := c.GetUint("employee_id"); empID != 0 {
		role = "employee"
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("signaling: upgrade failed:", err)
		return
	}

	r := getOrCreateRoom(loanID)
	cl := &client{conn: conn, room: r, role: role, send: make(chan []byte, 16)}
	r.evictRole(role, cl)
	r.join(cl)
	// Let the other party (if already connected) know someone joined —
	// the caller can use this to start the offer/answer exchange rather
	// than blindly polling.
	r.broadcastExcept(cl, []byte(`{"type":"peer-joined","role":"`+role+`"}`))

	done := make(chan struct{})
	go writePump(cl, done)
	readPump(cl, loanID, r, done)
}

func writePump(cl *client, done <-chan struct{}) {
	defer cl.conn.Close()
	// Server-side ping on top of the WS control frame layer (distinct from
	// the app-level {"type":"ping"} JSON keepalive the Flutter client also
	// sends) — this is what lets pongWait/SetReadDeadline below actually
	// detect a half-open TCP connection that neither side's app code would
	// otherwise notice for a long time.
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-cl.send:
			if !ok {
				return
			}
			if err := cl.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

type signalEnvelope struct {
	Type string `json:"type"`
}

func readPump(cl *client, loanID string, r *room, done chan<- struct{}) {
	defer func() {
		r.leave(cl)
		r.broadcastExcept(cl, []byte(`{"type":"peer-left","role":"`+cl.role+`"}`))
		dropRoomIfEmpty(loanID, r)
		close(done)
	}()

	cl.conn.SetReadDeadline(time.Now().Add(pongWait))
	cl.conn.SetPongHandler(func(string) error {
		cl.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := cl.conn.ReadMessage()
		if err != nil {
			return
		}

		var env signalEnvelope
		if err := json.Unmarshal(message, &env); err == nil {
			switch env.Type {
			case "ping":
				// App-level keepalive — proves this socket is alive, but has
				// no meaning to the other party, so it's swallowed here
				// rather than forwarded (previously it was relayed verbatim,
				// showing up as an unhandled message on the peer's side
				// every 20s during an active call for no functional reason).
				continue
			case "call-request":
				r.mu.Lock()
				othersPresent := len(r.clients) > 1
				r.mu.Unlock()
				if !othersPresent {
					// Nobody else is in the room to receive this — without
					// this, the caller was left "ringing" forever with no
					// signal that the other party was never connected
					// (e.g. customer app not running / not listening),
					// which is what made calls look like they silently
					// vanished into nothing.
					select {
					case cl.send <- []byte(`{"type":"peer-offline"}`):
					default:
					}
					continue
				}
			}
		}

		r.broadcastExcept(cl, message)
	}
}
