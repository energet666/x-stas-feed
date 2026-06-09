package game

import (
	"testing"
	"time"
)

func newTestWorld(t *testing.T) (*World, *time.Time) {
	t.Helper()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	w := NewWorld(nil)
	w.Close()
	w.now = func() time.Time { return now }
	w.randFloat = func() float64 { return 0.5 }
	return w, &now
}

func TestWorldAcceptsOnlyIncreasingCommandSequence(t *testing.T) {
	w, _ := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")

	if !w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 2, Input: Input{Thrust: true}}) {
		t.Fatal("expected first command to be accepted")
	}
	if w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 2}) {
		t.Fatal("expected duplicate command to be rejected")
	}
	if w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1}) {
		t.Fatal("expected older command to be rejected")
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.players[welcome.PlayerID].input.Thrust || w.players[welcome.PlayerID].ackSeq != 2 {
		t.Fatalf("unexpected player command state: %#v", w.players[welcome.PlayerID])
	}
}

func TestUnknownCommandDoesNotAdvanceAcknowledgement(t *testing.T) {
	w, _ := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	if w.Apply(welcome.PlayerID, Command{Type: "coordinates", Seq: 5}) {
		t.Fatal("expected unknown command to be rejected")
	}
	if !w.Apply(welcome.PlayerID, Command{Type: "heartbeat", Seq: 1}) {
		t.Fatal("rejected command must not consume its sequence number")
	}
}

func TestHeartbeatEchoesClientTimestampOnlyToPlayerState(t *testing.T) {
	w, _ := newTestWorld(t)
	first, _, _ := w.Connect("", "One")
	second, _, _ := w.Connect("", "Two")

	if !w.Apply(first.PlayerID, Command{Type: "heartbeat", Seq: 1, SentAtMS: 123456}) {
		t.Fatal("expected heartbeat to be accepted")
	}

	w.mu.Lock()
	snapshot := w.snapshotLocked(nil)
	w.mu.Unlock()
	for _, player := range snapshot.Players {
		switch player.ID {
		case first.PlayerID:
			if player.PingEcho != 123456 {
				t.Fatalf("expected heartbeat echo, got %#v", player)
			}
		case second.PlayerID:
			if player.PingEcho != 0 {
				t.Fatalf("expected other player echo to remain empty, got %#v", player)
			}
		}
	}
}

func TestWorldMovesShipAndLimitsItToArena(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	for range 300 {
		*now = now.Add(time.Second / tickRate)
		w.step()
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[welcome.PlayerID]
	if p.x < 0 || p.x >= ArenaWidth || p.y < 0 || p.y >= ArenaHeight {
		t.Fatalf("ship escaped arena: x=%f y=%f", p.x, p.y)
	}
	if speed := p.vx*p.vx + p.vy*p.vy; speed > maxSpeed*maxSpeed+0.001 {
		t.Fatalf("ship exceeded max speed: %f", speed)
	}
}

func TestTripleShotSpawnsThreeBullets(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	p := w.players[welcome.PlayerID]
	w.applyPowerUpLocked(p, "triple-shot", *now)
	w.mu.Unlock()
	w.Apply(welcome.PlayerID, Command{Type: "shoot", Seq: 2})

	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.bullets) != 3 {
		t.Fatalf("expected triple shot to create 3 bullets, got %d", len(w.bullets))
	}
}

func TestRapidFireUsesServerCooldown(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	p := w.players[welcome.PlayerID]
	w.applyPowerUpLocked(p, "rapid-fire", *now)
	w.mu.Unlock()
	w.Apply(welcome.PlayerID, Command{Type: "shoot", Seq: 2})
	w.Apply(welcome.PlayerID, Command{Type: "shoot", Seq: 3})

	w.mu.Lock()
	if len(w.bullets) != 1 {
		t.Fatalf("expected cooldown to reject immediate second shot, got %d bullets", len(w.bullets))
	}
	w.mu.Unlock()

	*now = now.Add(rapidFireCooldown)
	w.Apply(welcome.PlayerID, Command{Type: "shoot", Seq: 4})
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.bullets) != 2 {
		t.Fatalf("expected rapid fire shot after cooldown, got %d bullets", len(w.bullets))
	}
}

func TestShieldAbsorbsOneCrash(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[welcome.PlayerID]
	w.applyPowerUpLocked(p, "shield", *now)
	w.crashPlayerLocked(p, nil, p.x, p.y, *now)
	if !p.active || p.shield != 0 || p.score != 0 {
		t.Fatalf("expected shield to absorb crash, got %#v", p)
	}
	w.crashPlayerLocked(p, nil, p.x, p.y, now.Add(shipCollisionGrace))
	if p.active || p.score != -200 {
		t.Fatalf("expected next crash to destroy ship, got %#v", p)
	}
}

func TestDeathClearsAllPowerUps(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[welcome.PlayerID]
	p.shield = 1
	p.rapidFireUntil = now.Add(rapidFireDuration)
	p.tripleShotUntil = now.Add(tripleShotDuration)
	p.overdriveUntil = now.Add(overdriveDuration)
	p.lastShotAt = *now

	w.crashPlayerLocked(p, nil, p.x, p.y, *now)
	if !p.active || p.shield != 0 || p.rapidFireUntil.IsZero() || p.tripleShotUntil.IsZero() || p.overdriveUntil.IsZero() {
		t.Fatalf("expected shield hit to preserve remaining boosts, got %#v", p)
	}

	w.crashPlayerLocked(p, nil, p.x, p.y, now.Add(shipCollisionGrace))
	if p.active || p.shield != 0 || !p.rapidFireUntil.IsZero() || !p.tripleShotUntil.IsZero() || !p.overdriveUntil.IsZero() || !p.lastShotAt.IsZero() {
		t.Fatalf("expected death to clear all boosts, got %#v", p)
	}
}

func TestNovaDestroysCurrentAsteroidsAndAwardsSoloScore(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[welcome.PlayerID]
	if p.asteroid == nil {
		t.Fatal("expected active player asteroid")
	}
	w.applyPowerUpLocked(p, "nova", *now)
	if p.asteroid != nil || p.score != 100 {
		t.Fatalf("expected nova to clear asteroid and award score, got asteroid=%#v score=%d", p.asteroid, p.score)
	}
	if len(w.events) != 1 || w.events[0].Type != "asteroid-destroyed" {
		t.Fatalf("expected authoritative destruction event, got %#v", w.events)
	}
}

func TestNovaDestroysAllEnemyShipsAndBypassesShields(t *testing.T) {
	w, now := newTestWorld(t)
	ownerWelcome, _, _ := w.Connect("", "Nova")
	firstWelcome, _, _ := w.Connect("", "One")
	secondWelcome, _, _ := w.Connect("", "Two")
	w.Apply(ownerWelcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(firstWelcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(secondWelcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	owner := w.players[ownerWelcome.PlayerID]
	first := w.players[firstWelcome.PlayerID]
	second := w.players[secondWelcome.PlayerID]
	first.shield = 2
	first.rapidFireUntil = now.Add(rapidFireDuration)
	second.tripleShotUntil = now.Add(tripleShotDuration)

	w.applyPowerUpLocked(owner, "nova", *now)

	if !owner.active {
		t.Fatal("expected nova owner to survive")
	}
	if first.active || second.active {
		t.Fatalf("expected all enemies to be destroyed, first=%#v second=%#v", first, second)
	}
	if first.shield != 0 || !first.rapidFireUntil.IsZero() || !second.tripleShotUntil.IsZero() {
		t.Fatalf("expected enemy boosts to reset, first=%#v second=%#v", first, second)
	}
	if owner.kills != 2 || first.kills != -1 || second.kills != -1 {
		t.Fatalf("unexpected nova kill scores: owner=%d first=%d second=%d", owner.kills, first.kills, second.kills)
	}
	killEvents := 0
	for _, event := range w.events {
		if event.Type == "ship-kill" && event.ShooterID == owner.id {
			killEvents++
		}
	}
	if killEvents != 2 {
		t.Fatalf("expected two nova ship-kill events, got %#v", w.events)
	}
}

func TestPowerUpsSpawnAndExpire(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	*now = now.Add(powerUpSpawnInterval / 2)
	w.step()
	w.mu.Lock()
	if len(w.powerUps) != 1 {
		t.Fatalf("expected first power-up spawn, got %#v", w.powerUps)
	}
	var spawned PowerUp
	for _, powerUp := range w.powerUps {
		spawned = powerUp
	}
	w.nextPowerUpAt = now.Add(time.Hour)
	w.mu.Unlock()

	*now = time.UnixMilli(spawned.ExpiresAt)
	w.step()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.powerUps) != 0 {
		t.Fatalf("expected expired power-up to be removed, got %#v", w.powerUps)
	}
}

func TestPowerUpPickupAllowsNearMiss(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[welcome.PlayerID]
	p.asteroid = nil
	w.nextEntity++
	w.powerUps[w.nextEntity] = PowerUp{
		ID:        w.nextEntity,
		Kind:      "shield",
		X:         wrap(p.x+48, ArenaWidth),
		Y:         p.y,
		ExpiresAt: now.Add(powerUpLifetime).UnixMilli(),
	}

	w.detectCollisionsLocked(*now)
	if p.shield != 1 || len(w.powerUps) != 0 {
		t.Fatalf("expected nearby boost to be collected, shield=%d powerUps=%#v", p.shield, w.powerUps)
	}
}

func TestSecondActivePlayerSwitchesSoloToMultiplayer(t *testing.T) {
	w, _ := newTestWorld(t)
	first, _, _ := w.Connect("", "One")
	second, _, _ := w.Connect("", "Two")

	w.Apply(first.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.mu.Lock()
	w.players[first.PlayerID].score = 500
	w.mu.Unlock()
	w.Apply(second.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mode != "multiplayer" || !w.roundEndsAt.IsZero() {
		t.Fatalf("expected multiplayer without timer, mode=%q end=%v", w.mode, w.roundEndsAt)
	}
	if w.players[first.PlayerID].score != 0 || w.players[second.PlayerID].kills != 0 {
		t.Fatal("expected solo score and multiplayer kills to reset")
	}
}

func TestConnectedSpectatorDoesNotSwitchSoloToMultiplayer(t *testing.T) {
	w, _ := newTestWorld(t)
	spectator, _, _ := w.Connect("", "Spectator")
	player, _, _ := w.Connect("", "Pilot")

	w.Apply(player.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mode != "solo" || w.players[spectator.PlayerID].inGame {
		t.Fatalf("spectator changed game membership: mode=%q spectator=%#v", w.mode, w.players[spectator.PlayerID])
	}
	snapshot := w.snapshotLocked(nil)
	for _, state := range snapshot.Players {
		if state.ID == spectator.PlayerID && state.State != "spectator" {
			t.Fatalf("expected explicit spectator state, got %#v", state)
		}
	}
}

func TestInactiveSpectatorReceivesActiveRemoteWorld(t *testing.T) {
	w, _ := newTestWorld(t)
	spectator, snapshots, _ := w.Connect("", "Spectator")
	player, _, _ := w.Connect("", "Pilot")
	w.Apply(player.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	var snapshot Snapshot
	select {
	case snapshot = <-snapshots:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for spectator snapshot")
	}
	for len(snapshots) > 0 {
		snapshot = <-snapshots
	}

	var spectatorState, playerState *PlayerState
	for index := range snapshot.Players {
		switch snapshot.Players[index].ID {
		case spectator.PlayerID:
			spectatorState = &snapshot.Players[index]
		case player.PlayerID:
			playerState = &snapshot.Players[index]
		}
	}
	if spectatorState == nil || spectatorState.Active || spectatorState.State != "spectator" {
		t.Fatalf("expected connected inactive spectator, got %#v", spectatorState)
	}
	if playerState == nil || !playerState.Active || playerState.State != "active" {
		t.Fatalf("expected active remote player, got %#v", playerState)
	}
	if len(snapshot.Asteroids) != 1 || snapshot.Asteroids[0].OwnerID != player.PlayerID {
		t.Fatalf("expected active player's asteroid in spectator snapshot, got %#v", snapshot.Asteroids)
	}
}

func TestLeaveKeepsPlayerForGraceThenResetsEmptyWorld(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.mu.Lock()
	w.players[welcome.PlayerID].score = 300
	w.mu.Unlock()
	w.Apply(welcome.PlayerID, Command{Type: "leave", Seq: 2})

	w.mu.Lock()
	p := w.players[welcome.PlayerID]
	if w.mode != "solo" || w.status != "playing" || !p.inGame || p.active || p.score != 300 || playerState(p) != "away" {
		t.Fatalf("expected away player to retain the round during grace, mode=%q status=%q player=%#v", w.mode, w.status, p)
	}
	w.mu.Unlock()

	*now = now.Add(reconnectGrace + time.Millisecond)
	w.step()

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mode != "idle" || w.status != "idle" || p.inGame || playerState(p) != "spectator" {
		t.Fatalf("expected expired player to leave the game, mode=%q status=%q player=%#v", w.mode, w.status, p)
	}
}

func TestExpiredAwayPlayerReturnsRemainingParticipantToSolo(t *testing.T) {
	w, now := newTestWorld(t)
	first, _, _ := w.Connect("", "One")
	second, _, _ := w.Connect("", "Two")
	w.Apply(first.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(second.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(second.PlayerID, Command{Type: "leave", Seq: 2})

	*now = now.Add(reconnectGrace + time.Millisecond)
	w.step()

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mode != "solo" || w.status != "playing" || w.roundEndsAt.IsZero() {
		t.Fatalf("expected remaining participant to continue in solo, mode=%q status=%q end=%v", w.mode, w.status, w.roundEndsAt)
	}
	if !w.players[first.PlayerID].inGame || w.players[second.PlayerID].inGame {
		t.Fatalf("unexpected game membership after grace: first=%#v second=%#v", w.players[first.PlayerID], w.players[second.PlayerID])
	}
}

func TestSoloTimeoutSavesAuthoritativeScore(t *testing.T) {
	saved := make(chan int, 1)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	w := NewWorld(func(_ string, score int) error {
		saved <- score
		return nil
	})
	w.Close()
	w.now = func() time.Time { return now }
	w.randFloat = func() float64 { return 0.5 }
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.mu.Lock()
	w.players[welcome.PlayerID].score = 700
	w.mu.Unlock()

	now = now.Add(roundDuration)
	w.step()

	select {
	case score := <-saved:
		if score != 700 {
			t.Fatalf("expected saved score 700, got %d", score)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for score save")
	}
}

func TestFinishedSoloRoundStaysFinishedUntilRestart(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(welcome.PlayerID, Command{Type: "finish", Seq: 2})

	for range 10 {
		*now = now.Add(time.Second / tickRate)
		w.step()
	}

	w.mu.Lock()
	if w.status != "finished" || w.mode != "solo" {
		t.Fatalf("expected finished solo round to persist, mode=%q status=%q", w.mode, w.status)
	}
	w.mu.Unlock()

	w.Apply(welcome.PlayerID, Command{Type: "restart", Seq: 3})
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status != "playing" || !w.players[welcome.PlayerID].active {
		t.Fatalf("expected restart to start a new round, status=%q active=%v", w.status, w.players[welcome.PlayerID].active)
	}
}

func TestFinishedSoloRoundIgnoresLateGameplayCommands(t *testing.T) {
	w, _ := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(welcome.PlayerID, Command{Type: "finish", Seq: 2})

	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 3, Input: Input{Left: true, Thrust: true}})
	w.Apply(welcome.PlayerID, Command{Type: "shoot", Seq: 4})

	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[welcome.PlayerID]
	if w.status != "finished" || p.active || p.input != (Input{}) {
		t.Fatalf("late gameplay command restarted finished round: status=%q player=%#v", w.status, p)
	}
	if len(w.bullets) != 0 {
		t.Fatalf("late shoot created bullets in finished round: %#v", w.bullets)
	}
}

func TestFinishedSoloRoundCanRestartAfterLeavingLeaderboard(t *testing.T) {
	w, _ := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(welcome.PlayerID, Command{Type: "finish", Seq: 2})
	w.Apply(welcome.PlayerID, Command{Type: "leave", Seq: 3})
	w.Apply(welcome.PlayerID, Command{Type: "restart", Seq: 4})
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 5, Input: Input{Thrust: true}})

	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[welcome.PlayerID]
	if w.status != "playing" || w.mode != "solo" || !p.active || !p.input.Thrust || !p.awaySince.IsZero() {
		t.Fatalf("expected gameplay input to resume after explicit restart, status=%q mode=%q player=%#v", w.status, w.mode, p)
	}
}

func TestLeavingFinishedSoloRoundResetsWorldAfterGrace(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(welcome.PlayerID, Command{Type: "finish", Seq: 2})
	w.Apply(welcome.PlayerID, Command{Type: "leave", Seq: 3})

	w.mu.Lock()
	if w.status != "finished" || w.mode != "solo" {
		t.Fatalf("expected finished round during grace, mode=%q status=%q", w.mode, w.status)
	}
	w.mu.Unlock()

	*now = now.Add(reconnectGrace + time.Millisecond)
	w.step()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status != "idle" || w.mode != "idle" {
		t.Fatalf("expected expired leave to reset finished world, mode=%q status=%q", w.mode, w.status)
	}
}

func TestSoloCrashRespawnsInsideSameRound(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	p := w.players[welcome.PlayerID]
	p.score = 300
	asteroidID := p.asteroid.ID
	asteroidX := p.asteroid.X
	roundEndsAt := w.roundEndsAt
	w.crashPlayerLocked(p, nil, p.x, p.y, *now)
	w.mu.Unlock()

	for range 10 {
		*now = now.Add(time.Second / tickRate)
		w.step()
	}

	w.mu.Lock()
	if w.status != "playing" || w.mode != "solo" {
		t.Fatalf("expected solo round to continue after crash, mode=%q status=%q", w.mode, w.status)
	}
	if !w.roundEndsAt.Equal(roundEndsAt) {
		t.Fatalf("expected round deadline to stay unchanged, before=%v after=%v", roundEndsAt, w.roundEndsAt)
	}
	if p.score != 100 || !p.awaitingRespawn || p.active {
		t.Fatalf("unexpected crashed player state: score=%d awaiting=%v active=%v", p.score, p.awaitingRespawn, p.active)
	}
	if p.asteroid == nil || p.asteroid.ID != asteroidID || p.asteroid.X == asteroidX {
		t.Fatalf("expected crashed player's asteroid to remain simulated, got %#v", p.asteroid)
	}
	w.mu.Unlock()

	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 2, Input: Input{Thrust: true}})
	w.mu.Lock()
	defer w.mu.Unlock()
	if !p.active || p.awaitingRespawn {
		t.Fatalf("expected input to respawn player in same round: active=%v awaiting=%v", p.active, p.awaitingRespawn)
	}
	if !w.roundEndsAt.Equal(roundEndsAt) || p.score != 100 {
		t.Fatalf("respawn reset round state: end=%v score=%d", w.roundEndsAt, p.score)
	}
}

func TestDisconnectKeepsAwayPlayerAndAsteroidVisibleDuringGrace(t *testing.T) {
	w, now := newTestWorld(t)
	player, _, lease := w.Connect("", "Pilot")
	spectator, snapshots, _ := w.Connect("", "Spectator")
	w.Apply(player.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	w.players[player.PlayerID].score = 400
	asteroidX := w.players[player.PlayerID].asteroid.X
	w.mu.Unlock()
	w.Disconnect(player.PlayerID, lease)
	for range snapshotEveryTicks {
		*now = now.Add(time.Second / tickRate)
		w.step()
	}

	var snapshot Snapshot
	for len(snapshots) > 0 {
		snapshot = <-snapshots
	}
	var away *PlayerState
	for index := range snapshot.Players {
		if snapshot.Players[index].ID == player.PlayerID {
			away = &snapshot.Players[index]
			break
		}
	}
	if away == nil || away.State != "away" || away.Score != 400 {
		t.Fatalf("expected away player in spectator snapshot, got %#v for spectator %s", away, spectator.PlayerID)
	}
	if len(snapshot.Asteroids) != 1 || snapshot.Asteroids[0].X == asteroidX {
		t.Fatalf("expected away player's asteroid to keep moving, got %#v", snapshot.Asteroids)
	}
}

func TestReconnectWithinGraceRestoresPlayer(t *testing.T) {
	w, now := newTestWorld(t)
	first, _, lease := w.Connect("", "Pilot")
	w.Apply(first.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.mu.Lock()
	w.players[first.PlayerID].score = 250
	w.mu.Unlock()
	w.Disconnect(first.PlayerID, lease)
	*now = now.Add(9 * time.Second)

	resumed, _, _ := w.Connect(first.ResumeToken, "Pilot")
	if resumed.PlayerID != first.PlayerID || resumed.ResumeToken != first.ResumeToken {
		t.Fatalf("expected resumed session, got %#v", resumed)
	}
	w.mu.Lock()
	p := w.players[first.PlayerID]
	if p.score != 250 || playerState(p) != "away" {
		t.Fatalf("expected reconnect to preserve away state and score, got %#v", p)
	}
	w.mu.Unlock()

	w.Apply(first.PlayerID, Command{Type: "input", Seq: 2, Input: Input{Thrust: true}})
	w.mu.Lock()
	defer w.mu.Unlock()
	if !p.active || playerState(p) != "active" || p.score != 250 {
		t.Fatalf("expected control to resume the preserved player, got %#v", p)
	}
}

func TestReconnectAfterGraceCreatesNewPlayer(t *testing.T) {
	w, now := newTestWorld(t)
	first, _, lease := w.Connect("", "Pilot")
	w.Disconnect(first.PlayerID, lease)
	*now = now.Add(11 * time.Second)

	resumed, _, _ := w.Connect(first.ResumeToken, "Pilot")
	if resumed.PlayerID == first.PlayerID {
		t.Fatal("expected expired session to create a new player")
	}
}

func TestOldConnectionCannotDisconnectResumedSession(t *testing.T) {
	w, _ := newTestWorld(t)
	first, _, oldLease := w.Connect("", "Pilot")
	_, _, newLease := w.Connect(first.ResumeToken, "Pilot")

	w.Disconnect(first.PlayerID, oldLease)
	w.mu.Lock()
	if !w.players[first.PlayerID].connected {
		t.Fatal("old connection disconnected the resumed session")
	}
	w.mu.Unlock()

	w.Disconnect(first.PlayerID, newLease)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.players[first.PlayerID].connected {
		t.Fatal("current connection should disconnect the session")
	}
}

func TestToroidalDistanceUsesShortestEdgePath(t *testing.T) {
	if distance := toroidalDistance(2, 100, ArenaWidth-2, 100); distance != 4 {
		t.Fatalf("expected edge distance 4, got %f", distance)
	}
}
