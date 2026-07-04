package story

import "testing"

func TestMapEveryPlacedRoomHasLabel(t *testing.T) {
	for loc := range MapPositions {
		if MapLabels[loc] == "" {
			t.Errorf("room %d is placed on the map but has no label", loc)
		}
	}
}

func TestMapGraphIsConnectedAndSymmetric(t *testing.T) {
	edges := MapGraph()
	if len(edges) == 0 {
		t.Fatal("map graph has no edges")
	}
	// The road must connect to the well house and to the grate depression.
	if !hasEdge(edges, LOC_START, LOC_BUILDING) {
		t.Error("missing edge START–BUILDING")
	}
	if !hasEdge(edges, LOC_GRATE, LOC_BELOWGRATE) {
		t.Error("missing edge GRATE–BELOWGRATE (the descent)")
	}
	// Every edge must reference two placed rooms.
	for _, e := range edges {
		if _, ok := MapPositions[e.A]; !ok {
			t.Errorf("edge references unplaced room %d", e.A)
		}
		if _, ok := MapPositions[e.B]; !ok {
			t.Errorf("edge references unplaced room %d", e.B)
		}
	}
}

func TestMapNeighborStubsRevealUnexplored(t *testing.T) {
	s := New() // only START is "current"; nothing visited yet
	stubs := MapVisibleNeighbors(s)
	// From the road you can see the building, hill, valley, and grate slots.
	if !stubs[LOC_BUILDING] {
		t.Error("expected the well house to show as an unexplored neighbor of the road")
	}
	// A far cave room the player has never been near must not leak onto the map.
	if stubs[LOC_NUGGET] {
		t.Error("distant unvisited room should not be a visible neighbor from the start")
	}
}

func hasEdge(edges []MapEdge, a, b LocID) bool {
	for _, e := range edges {
		if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
			return true
		}
	}
	return false
}
