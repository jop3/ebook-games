package game

import "container/heap"

// ai.go: a heuristic Hex AI. Deep minimax is impractical (branching ~ N*N), so
// we score positions by "connection distance": the minimum number of empty
// cells a player must still fill to complete their edge-to-edge path (a
// Dijkstra where own stones cost 0, empty cost 1, opponent cells are walls).
// A player who needs fewer cells is closer to winning. The AI plays the move
// that most improves (its distance minus the opponent's distance), with
// immediate win/block shortcuts on top.

const infDist = 1 << 30

// connectionDistance returns the least number of empty cells player must still
// place to connect its two edges. 0 means already connected; infDist means the
// opponent has cut every path.
func connectionDistance(b *Board, player Cell) int {
	n := b.N
	dist := make([]int, n*n)
	for i := range dist {
		dist[i] = infDist
	}
	pq := &nodeHeap{}
	opp := player.Opponent()

	// cost to "enter" a cell: 0 if it's already ours, 1 if empty, wall if opp.
	cost := func(x, y int) int {
		switch b.At(x, y) {
		case player:
			return 0
		case opp:
			return infDist
		default:
			return 1
		}
	}

	// Seed the start edge.
	seed := func(x, y int) {
		c := cost(x, y)
		if c >= infDist {
			return
		}
		idx := y*n + x
		if c < dist[idx] {
			dist[idx] = c
			heap.Push(pq, node{x, y, c})
		}
	}
	if player == Black {
		for x := 0; x < n; x++ {
			seed(x, 0)
		}
	} else {
		for y := 0; y < n; y++ {
			seed(0, y)
		}
	}

	best := infDist
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(node)
		idx := cur.y*n + cur.x
		if cur.d > dist[idx] {
			continue
		}
		// Reached far edge?
		if (player == Black && cur.y == n-1) || (player == White && cur.x == n-1) {
			if cur.d < best {
				best = cur.d
			}
			continue
		}
		for _, dxy := range neighbors {
			nx, ny := cur.x+dxy[0], cur.y+dxy[1]
			if nx < 0 || nx >= n || ny < 0 || ny >= n {
				continue
			}
			c := cost(nx, ny)
			if c >= infDist {
				continue
			}
			nd := cur.d + c
			nidx := ny*n + nx
			if nd < dist[nidx] {
				dist[nidx] = nd
				heap.Push(pq, node{nx, ny, nd})
			}
		}
	}
	return best
}

// BestMove chooses a move for player. It first grabs an immediate win, then
// blocks an immediate opponent win, otherwise maximizes the connection-distance
// advantage. ok is false only if the board is full.
func BestMove(b *Board, player Cell) (move [2]int, ok bool) {
	empties := b.EmptyCells()
	if len(empties) == 0 {
		return move, false
	}
	opp := player.Opponent()

	// 1. Immediate win.
	for _, m := range empties {
		c := b.Clone()
		c.Set(m[0], m[1], player)
		if c.Winner() == player {
			return m, true
		}
	}
	// 2. Block an immediate opponent win.
	for _, m := range empties {
		c := b.Clone()
		c.Set(m[0], m[1], opp)
		if c.Winner() == opp {
			return m, true
		}
	}
	// 3. Heuristic: maximize (oppDist - myDist) after our move.
	bestScore := -infDist
	move = empties[0]
	ok = true
	for _, m := range empties {
		c := b.Clone()
		c.Set(m[0], m[1], player)
		my := connectionDistance(c, player)
		their := connectionDistance(c, opp)
		// Lower my distance is better; higher their distance is better.
		score := their - my
		if score > bestScore {
			bestScore = score
			move = m
		}
	}
	return move, ok
}

// --- priority queue for Dijkstra ------------------------------------------

type node struct {
	x, y, d int
}

type nodeHeap []node

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool  { return h[i].d < h[j].d }
func (h nodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x interface{}) { *h = append(*h, x.(node)) }
func (h *nodeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	it := old[n-1]
	*h = old[:n-1]
	return it
}
