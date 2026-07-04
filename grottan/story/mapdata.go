package story

// Map layout for the explored-map screen. This is hand-authored per story (the
// cave's geography here) rather than generated, and kept pure so the graph logic
// unit-tests without a device. A themed story (mystery/horror) drops in its own
// MapPos/MapLabel tables and reuses the same MapGraph/renderer unchanged.
//
// Positions are a coarse grid (Col east+, Row south+); the surface sits above
// the MapUndergroundRow divide, the cave below it. Nodes are revealed on the map
// only once the room has been visited, but their grid slots are fixed so the map
// fills in place instead of shifting as you explore.

// MapPos is a room's slot on the map grid.
type MapPos struct {
	Col, Row int
}

// MapCols / MapRows are the grid extents the renderer scales to fill the screen.
const (
	MapCols           = 4
	MapRows           = 7
	MapUndergroundRow = 4 // rows >= this are below the surface (the grate divide)
)

// MapPositions places each Phase-1 room. Rooms with no entry are simply omitted
// from the map.
var MapPositions = map[LocID]MapPos{
	LOC_ROADEND:     {0, 0},
	LOC_HILL:        {1, 0},
	LOC_START:       {2, 0},
	LOC_BUILDING:    {3, 0},
	LOC_VALLEY:      {2, 1},
	LOC_SLIT:        {2, 2},
	LOC_GRATE:       {2, 3},
	LOC_BELOWGRATE:  {2, 4},
	LOC_COBBLE:      {1, 4},
	LOC_DEBRIS:      {0, 4},
	LOC_AWKWARD:     {0, 5},
	LOC_BIRDCHAMBER: {1, 5},
	LOC_PITTOP:      {2, 5},
	LOC_MISTHALL:    {2, 6},
	LOC_NUGGET:      {1, 6},
}

// MapLabels are short Swedish tags shown in each map node (the transcript prose
// stays English; the map is chrome).
var MapLabels = map[LocID]string{
	LOC_ROADEND:     "Vägslut",
	LOC_HILL:        "Kullen",
	LOC_START:       "Vägen",
	LOC_BUILDING:    "Väghuset",
	LOC_VALLEY:      "Dalen",
	LOC_SLIT:        "Springan",
	LOC_GRATE:       "Gallret",
	LOC_BELOWGRATE:  "Under gallret",
	LOC_COBBLE:      "Kullersten",
	LOC_DEBRIS:      "Bråtrum",
	LOC_AWKWARD:     "Kanjon",
	LOC_BIRDCHAMBER: "Fågelrum",
	LOC_PITTOP:      "Gropkant",
	LOC_MISTHALL:    "Dimhall",
	LOC_NUGGET:      "Guldrum",
}

// MapEdge is an undirected structural connection between two rooms on the map.
type MapEdge struct {
	A, B LocID
}

// MapGraph returns the undirected adjacency (structural, ignoring conditions) of
// all placed rooms — every goto travel rule to another placed room becomes one
// edge, de-duplicated. Used to draw the corridors between map nodes.
func MapGraph() []MapEdge {
	seen := map[[2]LocID]bool{}
	var out []MapEdge
	for loc := range MapPositions {
		for _, dest := range roomNeighbors(loc) {
			if _, ok := MapPositions[dest]; !ok {
				continue
			}
			key := edgeKey(loc, dest)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, MapEdge{A: key[0], B: key[1]})
		}
	}
	return out
}

// roomNeighbors lists the in-subset goto destinations of a room (conditions
// ignored — this is geography, not current reachability).
func roomNeighbors(loc LocID) []LocID {
	var out []LocID
	seen := map[LocID]bool{}
	for _, t := range Locations[loc].Travel {
		if t.Act != ActGoto || t.Dest == LOC_NOWHERE {
			continue
		}
		if !seen[t.Dest] {
			seen[t.Dest] = true
			out = append(out, t.Dest)
		}
	}
	return out
}

func edgeKey(a, b LocID) [2]LocID {
	if a <= b {
		return [2]LocID{a, b}
	}
	return [2]LocID{b, a}
}

// MapVisibleNeighbors returns the placed rooms adjacent to a visited room that
// have NOT themselves been visited — drawn as hollow "unexplored" stubs so the
// player can see where passages still lead.
func MapVisibleNeighbors(s *State) map[LocID]bool {
	stubs := map[LocID]bool{}
	for loc := range MapPositions {
		if !s.Visited[loc] && loc != s.Loc {
			continue // only reveal neighbors of rooms we've actually been in
		}
		for _, dest := range roomNeighbors(loc) {
			if _, ok := MapPositions[dest]; !ok {
				continue
			}
			if !s.Visited[dest] && dest != s.Loc {
				stubs[dest] = true
			}
		}
	}
	return stubs
}
