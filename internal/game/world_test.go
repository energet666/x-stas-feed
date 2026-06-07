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
	if spectatorState == nil || spectatorState.Active {
		t.Fatalf("expected connected inactive spectator, got %#v", spectatorState)
	}
	if playerState == nil || !playerState.Active {
		t.Fatalf("expected active remote player, got %#v", playerState)
	}
	if len(snapshot.Asteroids) != 1 || snapshot.Asteroids[0].OwnerID != player.PlayerID {
		t.Fatalf("expected active player's asteroid in spectator snapshot, got %#v", snapshot.Asteroids)
	}
}

func TestLeaveResetsEmptyWorld(t *testing.T) {
	w, _ := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(welcome.PlayerID, Command{Type: "leave", Seq: 2})

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mode != "idle" || w.status != "idle" || !w.roundEndsAt.IsZero() {
		t.Fatalf("expected empty world to reset, mode=%q status=%q end=%v", w.mode, w.status, w.roundEndsAt)
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

func TestLeavingFinishedSoloRoundResetsWorld(t *testing.T) {
	w, _ := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})
	w.Apply(welcome.PlayerID, Command{Type: "finish", Seq: 2})
	w.Apply(welcome.PlayerID, Command{Type: "leave", Seq: 3})

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status != "idle" || w.mode != "idle" {
		t.Fatalf("expected leave to reset finished world, mode=%q status=%q", w.mode, w.status)
	}
}

func TestSoloCrashRespawnsInsideSameRound(t *testing.T) {
	w, now := newTestWorld(t)
	welcome, _, _ := w.Connect("", "Pilot")
	w.Apply(welcome.PlayerID, Command{Type: "input", Seq: 1, Input: Input{Thrust: true}})

	w.mu.Lock()
	p := w.players[welcome.PlayerID]
	p.score = 300
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

func TestReconnectWithinGraceRestoresPlayer(t *testing.T) {
	w, now := newTestWorld(t)
	first, _, lease := w.Connect("", "Pilot")
	w.Disconnect(first.PlayerID, lease)
	*now = now.Add(9 * time.Second)

	resumed, _, _ := w.Connect(first.ResumeToken, "Pilot")
	if resumed.PlayerID != first.PlayerID || resumed.ResumeToken != first.ResumeToken {
		t.Fatalf("expected resumed session, got %#v", resumed)
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
