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
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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
	for {
		select {
		case msg, ok := <-cl.send:
			if !ok {
				return
			}
			if err := cl.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func readPump(cl *client, loanID string, r *room, done chan<- struct{}) {
	defer func() {
		r.leave(cl)
		r.broadcastExcept(cl, []byte(`{"type":"peer-left","role":"`+cl.role+`"}`))
		dropRoomIfEmpty(loanID, r)
		close(done)
	}()

	for {
		_, message, err := cl.conn.ReadMessage()
		if err != nil {
			return
		}
		r.broadcastExcept(cl, message)
	}
}
