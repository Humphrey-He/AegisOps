package handler

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/Humphrey-He/AegisOps/internal/rbac"
	"github.com/Humphrey-He/AegisOps/internal/terminal"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TerminalHandler struct {
	service *terminal.Service
}

func NewTerminalHandler(service *terminal.Service) *TerminalHandler {
	return &TerminalHandler{service: service}
}

func (h *TerminalHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.POST("/hosts/:id/terminal/sessions", rbac.RequirePermission(rbacService, "terminal.open"), h.Create)
	r.GET("/terminal/sessions/:id", rbac.RequirePermission(rbacService, "terminal.open"), h.Get)
	r.POST("/terminal/sessions/:id/close", rbac.RequirePermission(rbacService, "terminal.open"), h.Close)
	r.GET("/terminal/sessions/:id/ws", rbac.RequirePermission(rbacService, "terminal.open"), h.Stream)
}

func (h *TerminalHandler) Create(c *gin.Context) {
	session, err := h.service.Create(c.Request.Context(), terminal.CreateRequest{
		HostID:    c.Param("id"),
		CreatedBy: OperatorID(c),
	})
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, session)
}

func (h *TerminalHandler) Get(c *gin.Context) {
	session, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, session)
}

func (h *TerminalHandler) Close(c *gin.Context) {
	if err := h.service.Close(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		Error(c, status, err.Error())
		return
	}
	OK(c, gin.H{"closed": true})
}

func (h *TerminalHandler) Stream(c *gin.Context) {
	if err := h.service.SessionExists(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		Error(c, status, err.Error())
		return
	}
	ws, err := upgradeWebSocket(c.Writer, c.Request)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.service.Stream(c.Request.Context(), c.Param("id"), ws)
}

type webSocketTerminal struct {
	conn    net.Conn
	reader  *bufio.Reader
	writeMu sync.Mutex
	pending []byte
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*webSocketTerminal, error) {
	if !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") ||
		!strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("websocket upgrade required")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" || r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, errors.New("invalid websocket handshake")
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("websocket hijack is not supported")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	accept := websocketAcceptKey(key)
	_, err = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &webSocketTerminal{conn: conn, reader: rw.Reader}, nil
}

func websocketAcceptKey(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (ws *webSocketTerminal) Read(p []byte) (int, error) {
	if len(ws.pending) > 0 {
		n := copy(p, ws.pending)
		ws.pending = ws.pending[n:]
		return n, nil
	}
	for {
		opcode, payload, err := ws.readFrame()
		if err != nil {
			return 0, err
		}
		switch opcode {
		case 0x1, 0x2:
			n := copy(p, payload)
			ws.pending = payload[n:]
			return n, nil
		case 0x8:
			return 0, io.EOF
		case 0x9:
			_ = ws.writeFrame(0xA, payload)
		}
	}
}

func (ws *webSocketTerminal) Write(p []byte) (int, error) {
	if err := ws.writeFrame(0x1, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (ws *webSocketTerminal) Close() error {
	_ = ws.writeFrame(0x8, nil)
	return ws.conn.Close()
}

func (ws *webSocketTerminal) readFrame() (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(ws.reader, header); err != nil {
		return 0, nil, err
	}
	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)
	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(ws.reader, ext); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(ws.reader, ext); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext)
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(ws.reader, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(ws.reader, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

func (ws *webSocketTerminal) writeFrame(opcode byte, payload []byte) error {
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()

	header := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126, byte(length>>8), byte(length))
	default:
		extended := make([]byte, 8)
		binary.BigEndian.PutUint64(extended, uint64(length))
		header = append(header, 127)
		header = append(header, extended...)
	}
	if _, err := ws.conn.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := ws.conn.Write(payload)
	return err
}
