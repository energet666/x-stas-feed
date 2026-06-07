package game

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	mathrand "math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ArenaWidth  = 1600.0
	ArenaHeight = 900.0

	tickRate             = 60
	snapshotEveryTicks   = 3
	roundDuration        = 60 * time.Second
	reconnectGrace       = 10 * time.Second
	shipCollisionGrace   = 1200 * time.Millisecond
	asteroidRespawnDelay = 650 * time.Millisecond
	maxNameRunes         = 40

	shipRadius     = 18.0
	shipNoseOffset = 25.0
	maxSpeed       = 8.5
	turnSpeed      = 0.105
	thrust         = 0.18
	drag           = 0.992
	bulletSpeed    = 11.0
	bulletLifetime = 82.0 / tickRate
)

type ScoreSaver func(name string, score int) error

type Input struct {
	Left   bool `json:"left"`
	Right  bool `json:"right"`
	Thrust bool `json:"thrust"`
}

type Command struct {
	Type     string `json:"type"`
	Seq      uint64 `json:"seq"`
	Input    Input  `json:"input,omitempty"`
	Name     string `json:"name,omitempty"`
	SentAtMS int64  `json:"sentAtMs,omitempty"`
}

type Welcome struct {
	Type        string   `json:"type"`
	PlayerID    string   `json:"playerId"`
	ResumeToken string   `json:"resumeToken"`
	Arena       Arena    `json:"arena"`
	Snapshot    Snapshot `json:"snapshot"`
}

type Arena struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Snapshot struct {
	Type        string        `json:"type"`
	Tick        uint64        `json:"tick"`
	Mode        string        `json:"mode"`
	Status      string        `json:"status"`
	RemainingMS int64         `json:"remainingMs"`
	Players     []PlayerState `json:"players"`
	Bullets     []BulletState `json:"bullets,omitempty"`
	Asteroids   []Asteroid    `json:"asteroids,omitempty"`
	Events      []Event       `json:"events,omitempty"`
}

type PlayerState struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	VX        float64 `json:"vx"`
	VY        float64 `json:"vy"`
	Angle     float64 `json:"angle"`
	Thrusting bool    `json:"thrusting"`
	Active    bool    `json:"active"`
	Score     int     `json:"score"`
	Kills     int     `json:"kills"`
	AckSeq    uint64  `json:"ackSeq"`
	PingEcho  int64   `json:"pingEcho,omitempty"`
}

type BulletState struct {
	ID      uint64  `json:"id"`
	OwnerID string  `json:"ownerId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	VX      float64 `json:"vx"`
	VY      float64 `json:"vy"`
}

type Asteroid struct {
	ID      uint64  `json:"id"`
	OwnerID string  `json:"ownerId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	VX      float64 `json:"vx"`
	VY      float64 `json:"vy"`
	Radius  float64 `json:"radius"`
	Angle   float64 `json:"angle"`
	Spin    float64 `json:"spin"`
	Path    string  `json:"path"`
}

type Event struct {
	ID          uint64  `json:"id"`
	Type        string  `json:"type"`
	X           float64 `json:"x,omitempty"`
	Y           float64 `json:"y,omitempty"`
	OwnerID     string  `json:"ownerId,omitempty"`
	ShooterID   string  `json:"shooterId,omitempty"`
	ShooterName string  `json:"shooterName,omitempty"`
	VictimID    string  `json:"victimId,omitempty"`
	VictimName  string  `json:"victimName,omitempty"`
	Saved       bool    `json:"saved,omitempty"`
}

type player struct {
	id              string
	token           string
	name            string
	x               float64
	y               float64
	vx              float64
	vy              float64
	angle           float64
	input           Input
	active          bool
	awaitingRespawn bool
	connected       bool
	disconnectedAt  time.Time
	collisionAfter  time.Time
	score           int
	kills           int
	ackSeq          uint64
	pingEcho        int64
	asteroid        *Asteroid
	respawnAt       time.Time
	sub             chan Snapshot
	lease           string
}

type bullet struct {
	BulletState
	age float64
}

type World struct {
	mu          sync.Mutex
	players     map[string]*player
	tokens      map[string]string
	bullets     map[uint64]*bullet
	mode        string
	status      string
	roundEndsAt time.Time
	tick        uint64
	nextEntity  uint64
	nextEvent   uint64
	events      []Event
	now         func() time.Time
	randFloat   func() float64
	saveScore   ScoreSaver
	stop        chan struct{}
}

func NewWorld(saveScore ScoreSaver) *World {
	w := &World{
		players:   make(map[string]*player),
		tokens:    make(map[string]string),
		bullets:   make(map[uint64]*bullet),
		mode:      "idle",
		status:    "idle",
		now:       time.Now,
		randFloat: mathrand.Float64,
		saveScore: saveScore,
		stop:      make(chan struct{}),
	}
	go w.run()
	return w
}

func (w *World) Close() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
}

func (w *World) Connect(resumeToken, name string) (Welcome, <-chan Snapshot, string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	w.cleanupLocked(now)
	var p *player
	if id := w.tokens[strings.TrimSpace(resumeToken)]; id != "" {
		candidate := w.players[id]
		if candidate != nil && (candidate.connected || now.Sub(candidate.disconnectedAt) <= reconnectGrace) {
			p = candidate
		}
	}
	if p == nil {
		id := randomID(16)
		token := randomID(24)
		x, y := spawnPoint(id, w.players)
		p = &player{id: id, token: token, name: normalizeName(name), x: x, y: y}
		w.players[id] = p
		w.tokens[token] = id
	}
	p.connected = true
	p.disconnectedAt = time.Time{}
	p.collisionAfter = now.Add(shipCollisionGrace)
	p.lease = randomID(12)
	if normalized := normalizeName(name); normalized != "Guest" || p.name == "" {
		p.name = normalized
	}
	p.sub = make(chan Snapshot, 4)
	snapshot := w.snapshotLocked(nil)
	return Welcome{
		Type:        "welcome",
		PlayerID:    p.id,
		ResumeToken: p.token,
		Arena:       Arena{Width: ArenaWidth, Height: ArenaHeight},
		Snapshot:    snapshot,
	}, p.sub, p.lease
}

func (w *World) Disconnect(playerID, lease string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[playerID]
	if p == nil || p.lease != lease {
		return
	}
	p.connected = false
	p.disconnectedAt = w.now()
	p.input = Input{}
	p.active = false
	deleteBulletsForOwner(w.bullets, p.id)
	p.sub = nil
	w.resetEmptyWorldLocked()
	w.publishLocked(nil)
}

func (w *World) Apply(playerID string, command Command) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	p := w.players[playerID]
	if p == nil || !p.connected || command.Seq <= p.ackSeq {
		return false
	}
	switch command.Type {
	case "input", "shoot", "restart", "finish", "leave", "name", "heartbeat":
	default:
		return false
	}
	p.ackSeq = command.Seq
	now := w.now()

	switch command.Type {
	case "input":
		p.input = command.Input
		if command.Input.Left || command.Input.Right || command.Input.Thrust {
			w.activateLocked(p, now)
		}
	case "shoot":
		w.activateLocked(p, now)
		if p.active && !hasBulletForOwner(w.bullets, p.id) {
			w.spawnBulletLocked(p)
		}
	case "restart":
		w.activateLocked(p, now)
	case "finish":
		if w.mode == "solo" && p.active {
			w.finishSoloLocked(p, false)
		}
	case "leave":
		p.input = Input{}
		p.active = false
		p.awaitingRespawn = false
		p.asteroid = nil
		deleteBulletsForOwner(w.bullets, p.id)
		if w.mode == "solo" {
			w.status = "idle"
		}
		w.resetEmptyWorldLocked()
	case "name":
		p.name = normalizeName(command.Name)
	case "heartbeat":
		if command.SentAtMS > 0 {
			p.pingEcho = command.SentAtMS
		}
	}
	w.publishLocked(nil)
	return true
}

func (w *World) run() {
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.step()
		case <-w.stop:
			return
		}
	}
}

func (w *World) step() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	w.tick++
	w.cleanupLocked(now)
	if w.status != "playing" {
		if w.tick%snapshotEveryTicks == 0 {
			w.publishLocked(nil)
		}
		return
	}
	if w.mode == "solo" && !w.roundEndsAt.IsZero() && !now.Before(w.roundEndsAt) {
		for _, p := range w.players {
			if p.active {
				w.finishSoloLocked(p, true)
				break
			}
		}
		return
	}

	for _, p := range w.players {
		if !p.connected || !p.active {
			continue
		}
		w.movePlayerLocked(p)
		if p.asteroid == nil && (p.respawnAt.IsZero() || !now.Before(p.respawnAt)) {
			w.spawnAsteroidLocked(p)
		}
		if p.asteroid != nil {
			p.asteroid.X = wrap(p.asteroid.X+p.asteroid.VX, ArenaWidth)
			p.asteroid.Y = wrap(p.asteroid.Y+p.asteroid.VY, ArenaHeight)
			p.asteroid.Angle += p.asteroid.Spin
		}
	}
	for id, b := range w.bullets {
		b.X = wrap(b.X+b.VX, ArenaWidth)
		b.Y = wrap(b.Y+b.VY, ArenaHeight)
		b.age += 1.0 / tickRate
		if b.age >= bulletLifetime {
			delete(w.bullets, id)
		}
	}
	w.detectCollisionsLocked(now)
	if w.tick%snapshotEveryTicks == 0 || len(w.events) > 0 {
		w.publishLocked(w.takeEventsLocked())
	}
}

func (w *World) activateLocked(p *player, now time.Time) {
	if p.active {
		return
	}
	if w.status != "playing" {
		w.mode = "solo"
		w.status = "playing"
		w.roundEndsAt = now.Add(roundDuration)
		for _, other := range w.players {
			other.score = 0
			other.kills = 0
		}
	} else if w.mode == "solo" {
		for _, other := range w.players {
			if other.active && other.id != p.id {
				w.mode = "multiplayer"
				w.roundEndsAt = time.Time{}
				for _, reset := range w.players {
					reset.score = 0
					reset.kills = 0
				}
				break
			}
		}
	}
	p.active = true
	p.awaitingRespawn = false
	p.collisionAfter = now.Add(shipCollisionGrace)
	p.respawnAt = time.Time{}
	if p.asteroid == nil {
		w.spawnAsteroidLocked(p)
	}
	if w.mode == "solo" {
		for _, other := range w.players {
			if other.active && other.id != p.id {
				w.mode = "multiplayer"
				w.roundEndsAt = time.Time{}
				for _, reset := range w.players {
					reset.score = 0
					reset.kills = 0
				}
				break
			}
		}
	}
}

func (w *World) movePlayerLocked(p *player) {
	if p.input.Left {
		p.angle -= turnSpeed
	}
	if p.input.Right {
		p.angle += turnSpeed
	}
	if p.input.Thrust {
		p.vx += math.Cos(p.angle) * thrust
		p.vy += math.Sin(p.angle) * thrust
	}
	speed := math.Hypot(p.vx, p.vy)
	if speed > maxSpeed {
		scale := maxSpeed / speed
		p.vx *= scale
		p.vy *= scale
	}
	p.vx *= drag
	p.vy *= drag
	p.x = wrap(p.x+p.vx, ArenaWidth)
	p.y = wrap(p.y+p.vy, ArenaHeight)
}

func (w *World) spawnBulletLocked(p *player) {
	w.nextEntity++
	w.bullets[w.nextEntity] = &bullet{BulletState: BulletState{
		ID:      w.nextEntity,
		OwnerID: p.id,
		X:       wrap(p.x+math.Cos(p.angle)*shipNoseOffset, ArenaWidth),
		Y:       wrap(p.y+math.Sin(p.angle)*shipNoseOffset, ArenaHeight),
		VX:      p.vx + math.Cos(p.angle)*bulletSpeed,
		VY:      p.vy + math.Sin(p.angle)*bulletSpeed,
	}}
}

func (w *World) spawnAsteroidLocked(p *player) {
	w.nextEntity++
	radius := 28 + w.randFloat()*16
	x, y := w.randomAsteroidPositionLocked(p, radius)
	p.asteroid = &Asteroid{
		ID:      w.nextEntity,
		OwnerID: p.id,
		X:       x,
		Y:       y,
		VX:      randomSigned(w.randFloat, 0.45, 1.15),
		VY:      randomSigned(w.randFloat, 0.25, 0.85),
		Radius:  radius,
		Angle:   w.randFloat() * math.Pi * 2,
		Spin:    randomSigned(w.randFloat, 0.008, 0.025),
		Path:    asteroidPath(w.randFloat, 14),
	}
}

func (w *World) randomAsteroidPositionLocked(p *player, radius float64) (float64, float64) {
	for range 12 {
		x := radius + w.randFloat()*(ArenaWidth-radius*2)
		y := 96 + radius + w.randFloat()*(ArenaHeight-96-radius*2)
		if toroidalDistance(x, y, p.x, p.y) > 220 {
			return x, y
		}
	}
	return ArenaWidth / 2, ArenaHeight / 2
}

func (w *World) detectCollisionsLocked(now time.Time) {
	for bulletID, b := range w.bullets {
		hit := false
		for _, owner := range w.players {
			if owner.asteroid == nil || !owner.connected || !owner.active {
				continue
			}
			if toroidalDistance(b.X, b.Y, owner.asteroid.X, owner.asteroid.Y) <= owner.asteroid.Radius+5 {
				shooter := w.players[b.OwnerID]
				if shooter != nil && w.mode == "solo" {
					shooter.score += 100
				}
				w.addEventLocked(Event{Type: "asteroid-destroyed", X: owner.asteroid.X, Y: owner.asteroid.Y, OwnerID: owner.id, ShooterID: b.OwnerID})
				owner.asteroid = nil
				owner.respawnAt = now.Add(asteroidRespawnDelay)
				delete(w.bullets, bulletID)
				hit = true
				break
			}
		}
		if hit {
			continue
		}
		for _, victim := range w.players {
			if victim.id == b.OwnerID || !victim.connected || !victim.active || now.Before(victim.collisionAfter) {
				continue
			}
			if toroidalDistance(b.X, b.Y, victim.x, victim.y) <= shipRadius+5 {
				w.crashPlayerLocked(victim, w.players[b.OwnerID], b.X, b.Y, now)
				delete(w.bullets, bulletID)
				break
			}
		}
	}

	for _, p := range w.players {
		if !p.connected || !p.active || now.Before(p.collisionAfter) {
			continue
		}
		for _, owner := range w.players {
			if owner.asteroid == nil || !owner.connected || !owner.active {
				continue
			}
			if toroidalDistance(p.x, p.y, owner.asteroid.X, owner.asteroid.Y) <= shipRadius+owner.asteroid.Radius {
				x, y := owner.asteroid.X, owner.asteroid.Y
				owner.asteroid = nil
				owner.respawnAt = now.Add(asteroidRespawnDelay)
				w.addEventLocked(Event{Type: "asteroid-destroyed", X: x, Y: y, OwnerID: owner.id})
				w.crashPlayerLocked(p, nil, x, y, now)
				break
			}
		}
	}

	active := make([]*player, 0, len(w.players))
	for _, p := range w.players {
		if p.connected && p.active && !now.Before(p.collisionAfter) {
			active = append(active, p)
		}
	}
	for i := 0; i < len(active); i++ {
		for j := i + 1; j < len(active); j++ {
			if !active[i].active || !active[j].active {
				continue
			}
			if toroidalDistance(active[i].x, active[i].y, active[j].x, active[j].y) <= shipRadius*2 {
				x := wrappedMidpoint(active[i].x, active[j].x, ArenaWidth)
				y := wrappedMidpoint(active[i].y, active[j].y, ArenaHeight)
				w.crashPlayerLocked(active[i], nil, x, y, now)
				w.crashPlayerLocked(active[j], nil, x, y, now)
			}
		}
	}
}

func (w *World) crashPlayerLocked(victim, shooter *player, x, y float64, now time.Time) {
	if !victim.active {
		return
	}
	victim.active = false
	victim.awaitingRespawn = true
	victim.input = Input{}
	victim.vx = 0
	victim.vy = 0
	victim.asteroid = nil
	deleteBulletsForOwner(w.bullets, victim.id)
	victim.x, victim.y = spawnPoint(victim.id, w.players)
	if w.mode == "solo" {
		victim.score -= 200
	} else {
		victim.kills--
		if shooter != nil {
			shooter.kills++
			w.addEventLocked(Event{Type: "ship-kill", X: x, Y: y, ShooterID: shooter.id, ShooterName: shooter.name, VictimID: victim.id, VictimName: victim.name})
			return
		}
	}
	w.addEventLocked(Event{Type: "ship-crash", X: x, Y: y, VictimID: victim.id, VictimName: victim.name})
}

func (w *World) finishSoloLocked(p *player, save bool) {
	p.active = false
	p.awaitingRespawn = false
	p.input = Input{}
	p.asteroid = nil
	deleteBulletsForOwner(w.bullets, p.id)
	w.status = "finished"
	w.roundEndsAt = time.Time{}
	w.addEventLocked(Event{Type: "round-finished", VictimID: p.id, Saved: save})
	score := p.score
	name := p.name
	if save && w.saveScore != nil {
		_ = w.saveScore(name, score)
	}
	events := w.takeEventsLocked()
	w.publishLocked(events)
}

func (w *World) cleanupLocked(now time.Time) {
	for id, p := range w.players {
		if p.connected || p.disconnectedAt.IsZero() || now.Sub(p.disconnectedAt) <= reconnectGrace {
			continue
		}
		delete(w.tokens, p.token)
		delete(w.players, id)
		deleteBulletsForOwner(w.bullets, id)
	}
	w.resetEmptyWorldLocked()
}

func (w *World) resetEmptyWorldLocked() {
	for _, p := range w.players {
		if p.active || p.awaitingRespawn || (w.status == "finished" && p.connected) || (!p.connected && !p.disconnectedAt.IsZero()) {
			return
		}
	}
	w.mode = "idle"
	w.status = "idle"
	w.roundEndsAt = time.Time{}
	w.bullets = make(map[uint64]*bullet)
}

func (w *World) addEventLocked(event Event) {
	w.nextEvent++
	event.ID = w.nextEvent
	w.events = append(w.events, event)
}

func (w *World) takeEventsLocked() []Event {
	events := append([]Event(nil), w.events...)
	w.events = nil
	return events
}

func (w *World) snapshotLocked(events []Event) Snapshot {
	now := w.now()
	players := make([]PlayerState, 0, len(w.players))
	asteroids := make([]Asteroid, 0, len(w.players))
	for _, p := range w.players {
		if !p.connected {
			continue
		}
		players = append(players, PlayerState{
			ID: p.id, Name: p.name, X: p.x, Y: p.y, VX: p.vx, VY: p.vy, Angle: p.angle,
			Thrusting: p.input.Thrust && p.active, Active: p.active, Score: p.score, Kills: p.kills, AckSeq: p.ackSeq,
			PingEcho: p.pingEcho,
		})
		if p.asteroid != nil {
			asteroids = append(asteroids, *p.asteroid)
		}
	}
	sort.Slice(players, func(i, j int) bool { return players[i].ID < players[j].ID })
	sort.Slice(asteroids, func(i, j int) bool { return asteroids[i].ID < asteroids[j].ID })
	bullets := make([]BulletState, 0, len(w.bullets))
	for _, b := range w.bullets {
		bullets = append(bullets, b.BulletState)
	}
	sort.Slice(bullets, func(i, j int) bool { return bullets[i].ID < bullets[j].ID })
	remaining := int64(0)
	if w.mode == "solo" && w.status == "playing" {
		remaining = max(0, w.roundEndsAt.Sub(now).Milliseconds())
	}
	return Snapshot{
		Type: "snapshot", Tick: w.tick, Mode: w.mode, Status: w.status, RemainingMS: remaining,
		Players: players, Bullets: bullets, Asteroids: asteroids, Events: events,
	}
}

func (w *World) publishLocked(events []Event) {
	snapshot := w.snapshotLocked(events)
	for _, p := range w.players {
		if p.sub == nil || !p.connected {
			continue
		}
		select {
		case p.sub <- snapshot:
		default:
			select {
			case <-p.sub:
			default:
			}
			select {
			case p.sub <- snapshot:
			default:
			}
		}
	}
}

func hasBulletForOwner(bullets map[uint64]*bullet, ownerID string) bool {
	for _, b := range bullets {
		if b.OwnerID == ownerID {
			return true
		}
	}
	return false
}

func deleteBulletsForOwner(bullets map[uint64]*bullet, ownerID string) {
	for id, b := range bullets {
		if b.OwnerID == ownerID {
			delete(bullets, id)
		}
	}
}

func normalizeName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "Guest"
	}
	runes := []rune(name)
	if len(runes) > maxNameRunes {
		return string(runes[:maxNameRunes])
	}
	return name
}

func randomID(bytesCount int) string {
	bytes := make([]byte, bytesCount)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))[:bytesCount*2]
	}
	return hex.EncodeToString(bytes)
}

func spawnPoint(id string, players map[string]*player) (float64, float64) {
	points := [][2]float64{{40, 40}, {1560, 40}, {40, 860}, {1560, 860}, {800, 40}, {800, 860}, {40, 450}, {1560, 450}}
	hash := uint32(0)
	for _, r := range id {
		hash = hash*31 + uint32(r)
	}
	start := int(hash % uint32(len(points)))
	for offset := range points {
		point := points[(start+offset)%len(points)]
		blocked := false
		for _, p := range players {
			if p.active && toroidalDistance(point[0], point[1], p.x, p.y) < shipRadius*4 {
				blocked = true
				break
			}
		}
		if !blocked {
			return point[0], point[1]
		}
	}
	return points[start][0], points[start][1]
}

func wrap(value, size float64) float64 {
	return math.Mod(math.Mod(value, size)+size, size)
}

func toroidalDistance(x1, y1, x2, y2 float64) float64 {
	dx := math.Abs(x1 - x2)
	dy := math.Abs(y1 - y2)
	dx = min(dx, ArenaWidth-dx)
	dy = min(dy, ArenaHeight-dy)
	return math.Hypot(dx, dy)
}

func wrappedMidpoint(a, b, size float64) float64 {
	if math.Abs(a-b) <= size/2 {
		return (a + b) / 2
	}
	return wrap((a+b+size)/2, size)
}

func randomSigned(random func() float64, minValue, maxValue float64) float64 {
	value := minValue + random()*(maxValue-minValue)
	if random() > 0.5 {
		return value
	}
	return -value
}

func asteroidPath(random func() float64, points int) string {
	path := ""
	for i := range points {
		angle := (float64(i) / float64(points)) * math.Pi * 2
		radius := 34 + random()*15
		x := 50 + math.Cos(angle)*radius
		y := 50 + math.Sin(angle)*radius
		command := "L"
		if i == 0 {
			command = "M"
		}
		path += command + " " + formatOneDecimal(x) + " " + formatOneDecimal(y) + " "
	}
	return strings.TrimSpace(path) + " Z"
}

func formatOneDecimal(value float64) string {
	rounded := math.Round(value*10) / 10
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", rounded), "0"), ".")
}
