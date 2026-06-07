package server

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"feed-ai/internal/game"
)

const (
	websocketGUID     = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	websocketText     = 0x1
	websocketClose    = 0x8
	websocketMaxFrame = 8192
	shipMessageLimit  = 120
)

func (s *Server) handleShipSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		writeError(w, http.StatusBadRequest, "websocket upgrade is required")
		return
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "websocket key is required")
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		writeError(w, http.StatusInternalServerError, "websocket upgrade is not supported")
		return
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	accept := websocketAccept(key)
	_, _ = fmt.Fprintf(
		rw,
		"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		accept,
	)
	if err := rw.Flush(); err != nil {
		return
	}

	welcome, snapshots, lease := s.ships.Connect(r.URL.Query().Get("resumeToken"), r.URL.Query().Get("name"))
	playerID := welcome.PlayerID
	defer s.ships.Disconnect(playerID, lease)

	done := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		data, err := json.Marshal(welcome)
		if err != nil || writeWebSocketText(conn, data) != nil {
			return
		}
		for {
			select {
			case <-done:
				return
			case snapshot, ok := <-snapshots:
				if !ok {
					return
				}
				data, err := json.Marshal(snapshot)
				if err != nil {
					continue
				}
				if err := writeWebSocketText(conn, data); err != nil {
					return
				}
			}
		}
	}()

	windowStarted := time.Now()
	messageCount := 0
	for {
		payload, err := readWebSocketText(conn)
		if err != nil {
			break
		}
		now := time.Now()
		if now.Sub(windowStarted) >= time.Second {
			windowStarted = now
			messageCount = 0
		}
		messageCount++
		if messageCount > shipMessageLimit {
			break
		}

		var command game.Command
		if err := json.Unmarshal(payload, &command); err != nil {
			continue
		}
		s.ships.Apply(playerID, command)
	}

	close(done)
	_ = conn.Close()
	<-writerDone
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		headerHasToken(r.Header.Get("Connection"), "upgrade") &&
		r.Header.Get("Sec-WebSocket-Version") == "13"
}

func headerHasToken(header string, token string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func websocketAccept(key string) string {
	hash := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func readWebSocketText(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	opcode := header[0] & 0x0f
	if header[0]&0x80 == 0 || header[0]&0x70 != 0 {
		return nil, errors.New("fragmented or reserved websocket frame")
	}
	masked := header[1]&0x80 != 0
	if !masked {
		return nil, errors.New("client websocket frame must be masked")
	}
	payloadLength := uint64(header[1] & 0x7f)
	switch payloadLength {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(conn, extended); err != nil {
			return nil, err
		}
		payloadLength = uint64(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(conn, extended); err != nil {
			return nil, err
		}
		payloadLength = binary.BigEndian.Uint64(extended)
	}
	if payloadLength > websocketMaxFrame {
		return nil, errors.New("websocket frame is too large")
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(conn, mask[:]); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	if opcode == websocketClose {
		return nil, io.EOF
	}
	if opcode != websocketText {
		return nil, errors.New("unsupported websocket frame")
	}

	return payload, nil
}

func writeWebSocketText(conn net.Conn, payload []byte) error {
	header := []byte{0x80 | websocketText}
	if len(payload) < 126 {
		header = append(header, byte(len(payload)))
	} else if len(payload) <= 0xffff {
		header = append(header, 126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(len(payload)))
	} else {
		header = append(header, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(len(payload)))
	}

	if _, err := conn.Write(header); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}
