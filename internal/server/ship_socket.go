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
)

const (
	websocketGUID     = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	websocketText     = 0x1
	websocketClose    = 0x8
	websocketMaxFrame = 8192
)

type shipSocketMessage struct {
	Type       string     `json:"type"`
	Ship       *shipState `json:"ship,omitempty"`
	OwnerID    string     `json:"ownerId,omitempty"`
	AsteroidID int        `json:"asteroidId,omitempty"`
	ShooterID  string     `json:"shooterId,omitempty"`
	X          float64    `json:"x,omitempty"`
	Y          float64    `json:"y,omitempty"`
}

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

	events := s.ships.subscribe()
	defer s.ships.unsubscribe(events)

	done := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for {
			select {
			case <-done:
				return
			case snapshot, ok := <-events:
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

	var shipID string
	for {
		payload, err := readWebSocketText(conn)
		if err != nil {
			break
		}

		var message shipSocketMessage
		if err := json.Unmarshal(payload, &message); err != nil {
			continue
		}
		switch message.Type {
		case "state", "":
			if message.Ship == nil {
				continue
			}
			if !sanitizeShipState(message.Ship) {
				continue
			}
			shipID = message.Ship.ID
			s.ships.update(*message.Ship)
		case "asteroid-hit":
			if shipID == "" {
				continue
			}
			s.ships.hitAsteroid(shipID, strings.TrimSpace(message.OwnerID), message.AsteroidID, message.X, message.Y)
		case "ship-kill":
			if shipID == "" {
				continue
			}
			s.ships.killShip(strings.TrimSpace(message.ShooterID), shipID, message.X, message.Y)
		case "ship-crash":
			if shipID == "" {
				continue
			}
			s.ships.crashShip(shipID, message.X, message.Y)
		}
	}

	if shipID != "" {
		s.ships.remove(shipID)
	}
	close(done)
	<-writerDone
}

func sanitizeShipState(ship *shipState) bool {
	ship.ID = strings.TrimSpace(ship.ID)
	ship.Name = strings.TrimSpace(ship.Name)
	if ship.ID == "" {
		return false
	}
	if ship.Name == "" {
		ship.Name = "Guest"
	}
	if nameRunes := []rune(ship.Name); len(nameRunes) > 40 {
		ship.Name = string(nameRunes[:40])
	}
	if !ship.Active {
		ship.Bullets = nil
		ship.Asteroid = nil
		ship.Thrusting = false
	}
	return true
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
	masked := header[1]&0x80 != 0
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
