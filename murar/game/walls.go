package game

import "image"

// overlaps reports whether wall w would overlap or cross any wall already on
// the board: two walls of the same orientation sharing (or immediately
// adjacent along the groove line to) the same anchor overlap, and a
// horizontal/vertical pair sharing the same anchor cross at their shared
// intersection.
func (b *Board) overlaps(w Wall) bool {
	switch w.Orient {
	case Horizontal:
		if b.WallH[w.Y][w.X] {
			return true
		}
		if w.X-1 >= 0 && b.WallH[w.Y][w.X-1] {
			return true
		}
		if w.X+1 < WallGrid && b.WallH[w.Y][w.X+1] {
			return true
		}
		return b.WallV[w.Y][w.X]
	default: // Vertical
		if b.WallV[w.Y][w.X] {
			return true
		}
		if w.Y-1 >= 0 && b.WallV[w.Y-1][w.X] {
			return true
		}
		if w.Y+1 < WallGrid && b.WallV[w.Y+1][w.X] {
			return true
		}
		return b.WallH[w.Y][w.X]
	}
}

// CanPlaceWall reports whether w may legally be placed on b: it must be in
// bounds, must not overlap or cross an existing wall, and — checked against
// the board AFTER the hypothetical placement — must leave BOTH players at
// least one path to their own goal row (a wall can trap the placer's own
// pawn just as easily as the opponent's, so both are checked independently).
func CanPlaceWall(b *Board, w Wall) bool {
	if w.X < 0 || w.X >= WallGrid || w.Y < 0 || w.Y >= WallGrid {
		return false
	}
	if b.overlaps(w) {
		return false
	}
	nb := *b
	nb.place(w)
	if _, ok := BFSDistance(&nb, nb.Pawns[P1], GoalRow(P1)); !ok {
		return false
	}
	if _, ok := BFSDistance(&nb, nb.Pawns[P2], GoalRow(P2)); !ok {
		return false
	}
	return true
}

// BFSDistance returns the length of the shortest path (in steps) from start
// to any cell on row goalRow, moving one orthogonal cell at a time and never
// crossing a wall. Pawn occupancy is deliberately ignored — a pawn can always
// eventually move out of the way, so wall legality and AI heuristics only
// care about wall-imposed connectivity, matching standard Quoridor path
// checks. ok is false if no such path exists.
func BFSDistance(b *Board, start image.Point, goalRow int) (int, bool) {
	if start.Y == goalRow {
		return 0, true
	}
	var visited [Size][Size]bool
	visited[start.Y][start.X] = true
	type node struct {
		p image.Point
		d int
	}
	queue := []node{{start, 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range dirs4 {
			next := cur.p.Add(d)
			if !inBounds(next) || visited[next.Y][next.X] || b.wallBetween(cur.p, next) {
				continue
			}
			if next.Y == goalRow {
				return cur.d + 1, true
			}
			visited[next.Y][next.X] = true
			queue = append(queue, node{next, cur.d + 1})
		}
	}
	return 0, false
}

// ShortestPath returns one shortest path (inclusive of start, exclusive of no
// particular endpoint beyond the first cell on goalRow reached) from start to
// goalRow, as a sequence of adjacent cells. ok is false if no path exists.
func ShortestPath(b *Board, start image.Point, goalRow int) ([]image.Point, bool) {
	if start.Y == goalRow {
		return []image.Point{start}, true
	}
	var visited [Size][Size]bool
	var parent [Size][Size]image.Point
	visited[start.Y][start.X] = true
	queue := []image.Point{start}
	var end image.Point
	found := false
	for len(queue) > 0 && !found {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range dirs4 {
			next := cur.Add(d)
			if !inBounds(next) || visited[next.Y][next.X] || b.wallBetween(cur, next) {
				continue
			}
			visited[next.Y][next.X] = true
			parent[next.Y][next.X] = cur
			if next.Y == goalRow {
				end = next
				found = true
				break
			}
			queue = append(queue, next)
		}
	}
	if !found {
		return nil, false
	}
	var path []image.Point
	for p := end; ; {
		path = append(path, p)
		if p == start {
			break
		}
		p = parent[p.Y][p.X]
	}
	// Reverse into start->end order.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, true
}
