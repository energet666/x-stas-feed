package server

import "testing"

func TestShipHubSuppressesDestroyedAsteroidUntilOwnerAcknowledges(t *testing.T) {
	hub := newShipHub()

	hub.update(testShipState("owner", testShipAsteroid(1)))
	hub.update(testShipState("shooter", nil))

	if _, ok := hub.hitAsteroid("shooter", "owner", 1, 100, 100); !ok {
		t.Fatal("expected asteroid hit to be accepted")
	}

	staleSnapshot := hub.update(testShipState("owner", testShipAsteroid(1)))
	if owner := findTestShip(t, staleSnapshot, "owner"); owner.Asteroid != nil {
		t.Fatalf("expected stale destroyed asteroid to stay suppressed, got %#v", owner.Asteroid)
	}

	hub.update(testShipState("owner", nil))
	reusedIDSnapshot := hub.update(testShipState("owner", testShipAsteroid(1)))
	if owner := findTestShip(t, reusedIDSnapshot, "owner"); owner.Asteroid == nil {
		t.Fatal("expected reused asteroid ID to be accepted after owner published no asteroid")
	}
}

func TestShipHubRemoveClearsDestroyedAsteroidHistory(t *testing.T) {
	hub := newShipHub()

	hub.update(testShipState("owner", testShipAsteroid(1)))
	hub.update(testShipState("shooter", nil))

	if _, ok := hub.hitAsteroid("shooter", "owner", 1, 100, 100); !ok {
		t.Fatal("expected asteroid hit to be accepted")
	}

	hub.remove("owner")
	reconnectedSnapshot := hub.update(testShipState("owner", testShipAsteroid(1)))
	if owner := findTestShip(t, reconnectedSnapshot, "owner"); owner.Asteroid == nil {
		t.Fatal("expected reconnected owner to be able to publish asteroid ID 1")
	}
}

func testShipState(id string, asteroid *shipAsteroid) shipState {
	return shipState{
		ID:       id,
		Name:     id,
		X:        40,
		Y:        60,
		Active:   true,
		Asteroid: asteroid,
	}
}

func testShipAsteroid(id int) *shipAsteroid {
	return &shipAsteroid{
		ID:     id,
		X:      100,
		Y:      100,
		Radius: 30,
		Angle:  0.5,
		Path:   "M 50 10 L 90 50 L 50 90 L 10 50 Z",
	}
}

func findTestShip(t *testing.T, snapshot shipSnapshot, id string) shipState {
	t.Helper()
	for _, ship := range snapshot.Ships {
		if ship.ID == id {
			return ship
		}
	}
	t.Fatalf("expected snapshot to include ship %q: %#v", id, snapshot.Ships)
	return shipState{}
}
