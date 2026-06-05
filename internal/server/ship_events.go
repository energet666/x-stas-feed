package server

import (
	"sync"
	"time"
)

const shipTTL = 8 * time.Second

type shipState struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	X         float64       `json:"x"`
	Y         float64       `json:"y"`
	Angle     float64       `json:"angle"`
	Thrusting bool          `json:"thrusting"`
	Active    bool          `json:"active"`
	Bullets   []shipBullet  `json:"bullets,omitempty"`
	Asteroid  *shipAsteroid `json:"asteroid,omitempty"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type shipBullet struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type shipAsteroid struct {
	ID     int     `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Angle  float64 `json:"angle"`
	Path   string  `json:"path"`
}

type shipSnapshot struct {
	Ships  []shipState `json:"ships"`
	Events []shipEvent `json:"events,omitempty"`
}

type shipEvent struct {
	Type        string  `json:"type"`
	OwnerID     string  `json:"ownerId,omitempty"`
	AsteroidID  int     `json:"asteroidId,omitempty"`
	ShooterID   string  `json:"shooterId,omitempty"`
	ShooterName string  `json:"shooterName,omitempty"`
	VictimID    string  `json:"victimId,omitempty"`
	VictimName  string  `json:"victimName,omitempty"`
	X           float64 `json:"x,omitempty"`
	Y           float64 `json:"y,omitempty"`
}

type shipHub struct {
	mu                 sync.Mutex
	ships              map[string]shipState
	destroyedAsteroids map[string]map[int]struct{}
	subscribers        map[chan shipSnapshot]struct{}
	now                func() time.Time
}

func newShipHub() *shipHub {
	return &shipHub{
		ships:              make(map[string]shipState),
		destroyedAsteroids: make(map[string]map[int]struct{}),
		subscribers:        make(map[chan shipSnapshot]struct{}),
		now:                time.Now,
	}
}

func (h *shipHub) subscribe() chan shipSnapshot {
	ch := make(chan shipSnapshot, 16)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.cleanupLocked()
	h.subscribers[ch] = struct{}{}
	ch <- h.snapshotLocked()

	return ch
}

func (h *shipHub) unsubscribe(ch chan shipSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subscribers, ch)
	close(ch)
}

func (h *shipHub) update(ship shipState) shipSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	ship.UpdatedAt = h.now().UTC()
	suppressedDestroyedAsteroid := false
	if ship.Asteroid != nil && h.isAsteroidDestroyedLocked(ship.ID, ship.Asteroid.ID) {
		ship.Asteroid = nil
		suppressedDestroyedAsteroid = true
	}
	if ship.Asteroid == nil && !suppressedDestroyedAsteroid {
		delete(h.destroyedAsteroids, ship.ID)
	}
	h.ships[ship.ID] = ship
	h.cleanupLocked()
	snapshot := h.snapshotLocked()
	h.publishLocked(snapshot)

	return snapshot
}

func (h *shipHub) hitAsteroid(shooterID, ownerID string, asteroidID int, bulletX, bulletY float64) (shipSnapshot, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, shooterOK := h.ships[shooterID]
	target, targetOK := h.ships[ownerID]
	if !shooterOK || !targetOK || target.Asteroid == nil || target.Asteroid.ID != asteroidID {
		return shipSnapshot{}, false
	}

	if distanceSquared(bulletX, bulletY, target.Asteroid.X, target.Asteroid.Y) > square(target.Asteroid.Radius+5) {
		return shipSnapshot{}, false
	}

	event := shipEvent{
		Type:       "asteroid-destroyed",
		OwnerID:    ownerID,
		AsteroidID: asteroidID,
		X:          target.Asteroid.X,
		Y:          target.Asteroid.Y,
	}
	target.Asteroid = nil
	h.ships[ownerID] = target
	h.markAsteroidDestroyedLocked(ownerID, asteroidID)
	h.cleanupLocked()
	snapshot := h.snapshotLocked()
	snapshot.Events = []shipEvent{event}
	h.publishLocked(snapshot)

	return snapshot, true
}

func (h *shipHub) killShip(shooterID, victimID string, x, y float64) (shipSnapshot, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	shooter, shooterOK := h.ships[shooterID]
	victim, victimOK := h.ships[victimID]
	if !shooterOK || !victimOK || shooterID == victimID {
		return shipSnapshot{}, false
	}

	event := shipEvent{
		Type:        "ship-kill",
		ShooterID:   shooterID,
		ShooterName: shooter.Name,
		VictimID:    victimID,
		VictimName:  victim.Name,
		X:           x,
		Y:           y,
	}
	h.cleanupLocked()
	snapshot := h.snapshotLocked()
	snapshot.Events = []shipEvent{event}
	h.publishLocked(snapshot)

	return snapshot, true
}

func (h *shipHub) crashShip(victimID string, x, y float64) (shipSnapshot, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	victim, victimOK := h.ships[victimID]
	if !victimOK {
		return shipSnapshot{}, false
	}

	event := shipEvent{
		Type:       "ship-crash",
		VictimID:   victimID,
		VictimName: victim.Name,
		X:          x,
		Y:          y,
	}
	h.cleanupLocked()
	snapshot := h.snapshotLocked()
	snapshot.Events = []shipEvent{event}
	h.publishLocked(snapshot)

	return snapshot, true
}

func (h *shipHub) remove(id string) shipSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.ships, id)
	delete(h.destroyedAsteroids, id)
	h.cleanupLocked()
	snapshot := h.snapshotLocked()
	h.publishLocked(snapshot)

	return snapshot
}

func (h *shipHub) cleanupLocked() {
	cutoff := h.now().Add(-shipTTL)
	for id, ship := range h.ships {
		if ship.UpdatedAt.Before(cutoff) {
			delete(h.ships, id)
			delete(h.destroyedAsteroids, id)
		}
	}
}

func (h *shipHub) snapshotLocked() shipSnapshot {
	ships := make([]shipState, 0, len(h.ships))
	for _, ship := range h.ships {
		ships = append(ships, ship)
	}
	return shipSnapshot{Ships: ships}
}

func (h *shipHub) isAsteroidDestroyedLocked(ownerID string, asteroidID int) bool {
	asteroids := h.destroyedAsteroids[ownerID]
	if asteroids == nil {
		return false
	}
	_, ok := asteroids[asteroidID]
	return ok
}

func (h *shipHub) markAsteroidDestroyedLocked(ownerID string, asteroidID int) {
	if h.destroyedAsteroids[ownerID] == nil {
		h.destroyedAsteroids[ownerID] = make(map[int]struct{})
	}
	h.destroyedAsteroids[ownerID][asteroidID] = struct{}{}
}

func (h *shipHub) publishLocked(snapshot shipSnapshot) {
	for ch := range h.subscribers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func distanceSquared(x1, y1, x2, y2 float64) float64 {
	dx := x1 - x2
	dy := y1 - y2
	return (dx*dx + dy*dy)
}

func square(value float64) float64 {
	return value * value
}
