# Build spec — E-Reader Strategy Game Library candidates

Hand-off spec for the batch evaluated from the user-supplied "E-Reader Strategy Game Library
Candidates" list (28 titles). Companion to `GAME_FEASIBILITY_EVAL.md` (the previous batch, now
fully shipped as `2048`, `hasami`, `shong`, `munkar`, `sushi`, `mosaik`, `ringar`, `goban`,
`hexa`). This doc grades the **new** list and specs everything worth building.

**Already built — skip:**
- **Azul** → shipped as `mosaik` (commit `1d77405`).
- **YINSH** → shipped as `ringar` (commit `36b20ee`).

**Skip (redundant or poor fit) — no spec below:**
- **Robot Turtles** — "program moves, execute sequentially" is exactly what `roborally` already
  does; building this would just be a simpler reskin.
- **Jaipur, Schotten Totten/Battle Line, Century: Spice Road, Kingdomino** — each is a strong game,
  but each duplicates a niche another pick below already fills better (see "one pick per niche"
  note under Batch 3). Listed as alternates, not built.
- **Ricochet Robots, Rush Hour** — both are the "slide until blocked" mechanic that
  `POCKETBOOK_GAMEDEV_GUIDE.md` (§12, citing PocketPuzzles' own source) explicitly names as
  excluded on this hardware class ("Inertia" — eInk rendering/animation problems). A turn-based,
  no-animation, before/after-board-only treatment (the same defense that saved `2048`) might work,
  but it's an open risk, not a validated one — deprioritized behind everything else here. If
  picked up later, treat as a **solo generated puzzle** (optimal-move-count target, no AI opponent)
  rather than a competitive game, and prototype the render/tap-hit-testing first.

**Batches (build low-risk/high-value first):**

| # | Game | Fit | Effort | Notes |
|---|---|---|---|---|
| 1 | **Quoridor** | ★★★★★ | Medium | wall-blocking race, flagship pick |
| 1 | **Breakthrough** | ★★★★★ | Low | pawn race, cheap strong AI |
| 1 | **Domineering** | ★★★★☆ | Low | tiny combinatorial game, near-perfect AI |
| 1 | **Chomp** | ★★★★☆ | Low | tiny impartial game, perfect-play AI possible |
| 1 | **L Game** | ★★★★☆ | Low | 4×4 fixed board, tiny state space |
| 1 | **Cathedral** | ★★★★☆ | Medium | territory enclosure, novel mechanic |
| 2 | **Ataxx** | ★★★★☆ | Low–Medium | clone/jump infection, clean fit |
| 2 | **Konane** | ★★★★☆ | Low–Medium | jump-only capture, forces-a-move angle |
| 2 | **Breakthrough**-adjacent: **Isola** | ★★★★☆ | Low–Medium | mobility-heuristic classic |
| 2 | **The Duke** | ★★★☆☆ | Medium | data-driven per-tile move tables |
| 2 | **TZAAR** | ★★★☆☆ | Medium | stacking capture, hex board |
| 2 | **Amazons** | ★★★☆☆ | Medium–High | huge branching, AI is the risk |
| 2 | **Hnefatafl** | ★★★☆☆ | Medium–High | asymmetric; de-risk with a small variant first |
| 3 | **Hanamikoji** | ★★★★☆ | Medium | hidden-info, tiny state, 2nd card game |
| 3 | **Lost Cities** | ★★★★☆ | Medium | hidden-info, iconic, 2-player |
| 3 | **Splendor** | ★★★★☆ | Medium | perfect-info engine builder |
| 3 | **Patchwork** | ★★★★☆ | Medium | 2-player spatial + economy |

Every game follows the **same non-negotiable setup as `SPEC_NEXT_GAMES.md` §0 /
`SPEC_BUILD_GAMES.md` §0** — read those first, and read `POCKETBOOK_GAMEDEV_GUIDE.md` in full
before writing code. Summary of what carries over unchanged:

- Copy the closest existing module (**`hasami/` is the best template for every 2-player
  perfect-info board game below** — board + tap-select-then-destination + `aiPend`-after-paint AI
  scaffolding; `sushi/` for the hidden-info card games in Batch 3).
- All rules/AI logic in `game/` (no `ink` import), unit-tested via `go test ./game/`.
- `screenSplash → screenMenu → screenGame → screenRules` state machine; splash motif; full Swedish
  rules text; hardware Back returns to menu; `ink.Repaint()` at end of `Init()`.
- `aiPend`-after-paint pattern (compute the AI move only after the human's move has been drawn).
- `play_test.go` (`//go:build playtest`) driving a full game through real taps via
  `playtest/play.sh <game>`; delete any `*_render_test.go` before shipping.
- ARM `.app` via `sunsung/pocketbook-go-sdk:latest` (guide §7); the repo's `build.yml`/`ci.yml`
  auto-discover any new top-level Go module, so **no workflow-file edits are needed** for a new
  game.
- **Naming/attribution courtesy** (per `SPEC_BUILD_GAMES.md`): rules aren't copyrightable, but
  names/art/themes are. Traditional or academic games (Breakthrough, Domineering, Chomp, Konane,
  Amazons, Isola, Ataxx) can keep their real names — same treatment as `othello`/`hex`/`nim`.
  Actively-published commercial games (Quoridor/Gigamic, Cathedral/Gamewright, TZAAR/GIPF,
  The Duke/Catalyst, Hanamikoji/EmperorS4, Lost Cities/Splendor/Patchwork — all Kosmos/Days of
  Wonder/Space Cowboys-family) should get a neutral Swedish working title + original art, with a
  "Baserat på …" credit line on the rules screen — same treatment as `munkar`/`ringar`/`mosaik`.
  Suggested titles are given per game below; treat them as placeholders, not final.

---

# BATCH 1 — build first

## GAME 1 — Quoridor  (suggested title: **"Murar"** / Walls)

**Elevator pitch:** race your pawn to the far side of the board — or spend your turn building a
wall to slow your opponent down instead. You can never wall someone in completely.

### Rules (Swedish on the rules screen)
- 9×9 board. Each player has 1 pawn, starting centered on their own edge; goal = reach **any**
  cell of the opposite edge. Each player holds **10 walls**.
- On your turn, do exactly one of:
  - **Move** your pawn one step orthogonally into an empty cell.
  - **Jump**: if the adjacent cell in your move direction is occupied by the opponent, and the
    cell directly beyond it (same direction) is empty and not wall-blocked, you jump there. If
    that beyond-cell is blocked (wall or board edge), you may instead step **diagonally** to
    either open cell adjacent to the opponent (the classic Quoridor jump exception).
  - **Place a wall**: a 2-cell-long segment in the "groove" grid (8×8 possible slots, horizontal or
    vertical), which must not overlap or cross an existing wall at the same intersection, and
    **must leave both players at least one path to their own goal edge** (verify by pathfinding
    after every placement; reject illegal walls outright, don't just warn).
- **Win:** first pawn to reach the opposite edge.

### game/ model + logic (ink-free, unit-tested)
- `Board`: 9×9 pawn positions + wall state as two 8×8 bool grids (`WallH`, `WallV`).
- `LegalPawnMoves(b, pos)`: 4 orthogonal steps filtered by walls/edges, plus the jump/diagonal
  exception above (test all 4 approach directions and both diagonal fallbacks explicitly).
- `CanPlaceWall(b, wall) bool`: overlap/cross check, then BFS/flood-fill from each pawn to its goal
  row over the post-placement wall state — reject if either path is empty.
- `Winner(b)`: either pawn on its goal edge.

### AI — the real risk, be honest about it
- Naive full-width alpha-beta is not viable: each turn has ~4 pawn moves **plus** up to ~120 wall
  slots. Standard mitigation (used by most hobby Quoridor engines, apply here):
  1. **Evaluate with BFS shortest-path distance** to each player's goal edge (own distance −
     opponent distance, weighted); this alone gives a competent-feeling opponent even at depth 1.
  2. **Prune the wall-move candidate set** to walls adjacent to the opponent's current shortest
     path (recompute after every hypothetical placement) rather than enumerating all ~120 slots.
  3. Shallow minimax (depth 1–2) over {pawn moves} ∪ {pruned wall candidates}.
  - Ship this as a "plays reasonably, not a master" AI and say so on the rules/menu screen — same
    honesty policy as the `goban` Go AI.

### UI
- 9×9 grid, generous margins for wall grooves. Toggle button "Drag pjäs / Bygg mur" (move mode /
  wall mode). Move mode: tap an empty highlighted destination cell. Wall mode: tap a groove
  intersection to preview, tap again (or a second orientation button) to confirm; illegal
  placements simply don't confirm (redraw with a rejection flash).
- Pawns = filled disc vs outline ring; walls = thick black bar across the groove.
- **Splash motif:** two pawns at opposite short edges of a small grid with one short wall segment
  between them.

### Gotchas
- The diagonal-jump exception is the single most-forgotten rule — unit-test both the "beyond cell
  blocked by wall" and "beyond cell is off-board" triggers separately.
- "Must leave a path" has to run **after** a hypothetical wall is placed, for **both** players
  independently — a wall can trap the placer's own pawn too.
- Don't promise a strong AI; ship the heuristic version and label it honestly (mirrors `goban`).

---

## GAME 2 — Breakthrough (keep the name — academic public-domain game)

**Elevator pitch:** pawns only, no chess baggage — move straight, capture diagonally, first pawn
across the board wins.

### Rules
- 8×6 (or 8×8 — pick one; 8×6 keeps games shorter and the board fits the portrait screen with room
  to spare) board, each side fills their 2 nearest ranks with pawns.
- A pawn moves **one step straight forward** onto an empty square, or **one step diagonally
  forward** onto a square with an enemy pawn (capturing it) — the reverse of chess pawns: no
  diagonal-only-when-capturing ambiguity, straight is always non-capturing, diagonal is always
  capturing. No double-step, no en passant, no promotion.
- **Win:** get one pawn onto the opponent's home rank, or eliminate all opposing pawns, or the
  opponent has no legal move.

### game/ model + logic
- Trivial board array + per-side move generator (2 forward-diagonal capture cells, 1 forward
  non-capture cell, bounds-checked).
- `Winner`: home-rank reached / one side has 0 pawns / no legal moves for the side to move.

### AI
- Well-studied small game (solved on smaller boards); alpha-beta with material + advancement
  (reward pawns closer to the goal rank, especially unopposed ones) is cheap and strong. Depth
  6–8 is comfortable at this board size — the standout "genuinely strong, genuinely cheap" AI in
  this batch, same story as `shong`.

### UI
- Reuse `hasami`'s disc rendering (filled vs outline). Tap own pawn → highlighted destinations →
  tap. **Splash motif:** a pawn stepping diagonally past an opposing pawn with a small capture "×".

### Gotchas
- Direction is mirror-imaged per side — don't hardcode "up" without checking whose pawn it is.
- Getting "straight = never captures, diagonal = always captures" backwards is the classic bug vs.
  chess muscle memory; unit-test both explicitly for both sides.

---

## GAME 3 — Domineering (keep the name — Conway's academic game)

**Elevator pitch:** place your dominoes — one player only vertical, the other only horizontal —
until someone can't move. That player loses.

### Rules
- 8×8 board (classic; a smaller 6×6 is a fine "Lätt" option). Player V may only place a domino
  **vertically** (covering two cells in the same column); Player H only **horizontally**. Turns
  alternate; a player who cannot legally place their domino loses (normal play convention).

### game/ model + logic
- Board = bool grid (occupied/empty). `LegalMoves(b, side)`: every pair of adjacent empty cells in
  the side's fixed orientation.
- `Winner`: whoever is to move with zero legal moves loses.

### AI
- Board shrinks every move (branching strictly decreases), so alpha-beta can search deep, even to
  the end in the endgame. Eval = own-legal-moves − opponent-legal-moves (mobility) is already
  strong for this game family; add a light positional bias if time allows. Cheapest "actually
  strong" AI in the batch alongside Breakthrough.

### UI
- Grid with a "ghost" domino preview following taps: tap one cell, the second (fixed-orientation)
  cell auto-highlights, tap again to confirm (no freeform 2-tap-any-direction — orientation is
  fixed by whose turn it is, which simplifies input a lot).
- **Splash motif:** a small grid with 2–3 vertical and horizontal domino outlines mid-game.

### Gotchas
- Don't let a player attempt the other orientation — legality must check the *mover's* fixed
  orientation only, not "any adjacent pair."
- Misère-vs-normal play confusion is the classic bug in every combinatorial-game port: this is
  **normal play** (last player to move **wins**, i.e. the player who **can't** move loses).

---

## GAME 4 — Chomp (keep the name — public-domain mathematical game)

**Elevator pitch:** a rectangular chocolate bar with a poisoned corner square. Take a bite — it
removes everything to the right of and below your pick, too. Whoever's forced to eat the poison
loses.

### Rules
- Grid (e.g. 5×6). Top-left cell is the poisoned square. On your turn, pick any remaining cell;
  it and every remaining cell with row ≥ its row **and** column ≥ its column are removed. Whoever
  is forced to take the poisoned square loses.

### game/ model + logic
- State is always a "staircase" — represent as one int per row = how many cells remain in that
  row (monotonically non-increasing top to bottom). A move picks (row, col): every row ≥ that row
  gets clamped to min(current length, col).
- **State space is tiny** (a monotone partition of the grid) — for anything up to ~7×8, full
  memoized minimax is instant and gives a **perfect-play AI** (unbeatable, like the existing `nim`
  AI), not just "strong."

### AI
- Exhaustive memoized minimax on the staircase representation. No heuristics needed — ship the
  perfect version and say so, same honesty-in-the-other-direction as `nim`.

### UI
- Tap any remaining cell to eat it + its removed region (grey out immediately, no animation
  needed — direct before/after redraw, same defense as `2048`).
- **Splash motif:** a chocolate-bar grid with a corner bite taken out (staircase shape) and a
  skull/mark on the single poisoned square.

### Gotchas
- Off-by-one on "row ≥ / col ≥" direction (which corner is poison, which direction the bite
  extends) is the classic bug — unit-test the exact removed-region shape for a few hand-picked
  moves.

---

## GAME 5 — L Game (keep the name — de Bono's academic puzzle-game)

**Elevator pitch:** a 4×4 board, one L-shaped piece each, two tiny neutral blockers. Move your L
somewhere new, optionally nudge a neutral piece — corner your opponent so their L has nowhere
legal to go and you win.

### Rules
- Fixed 4×4 board. Each player has one L-tetromino piece (4 cells, can be placed in any of its 8
  rotation/reflection orientations) and there are 2 neutral single-cell pieces shared on the board.
- On your turn: **lift your own L piece and place it anywhere** (any orientation) on the board as
  long as the new position differs from the old and doesn't overlap any other piece — this is
  **mandatory**, you must move your L if any legal placement exists. Then, **optionally**, move
  either neutral piece to any other empty cell.
- **Win:** if, on your turn, you have **no legal placement** for your L piece, you lose.

### game/ model + logic
- Board = 16 cells; L piece = one of 8 orientation shapes anchored at a cell; enumerate all legal
  placements (excluding current position) as the move generator.
- `Winner`: side to move with zero legal L placements loses.

### AI
- Tiny total state space (16-cell board, 3 pieces) — full-depth alpha-beta (or even exhaustive
  search) is cheap; this can be a very strong, close-to-optimal opponent for almost no engineering
  effort, similar in spirit to `shong`'s "small board → strong AI is trivial" story.

### UI
- Tap-select one of the 8 L orientations (small orientation picker) → tap an anchor cell to place
  (only legal anchors highlighted) → then optionally tap a neutral piece and its new cell.
- **Splash motif:** a 4×4 grid with an L-piece in one corner and 2 small neutral dots.

### Gotchas
- "Must move if a legal placement exists" is the whole win condition — a buggy move generator that
  misses a legal placement will falsely end the game.
- 8 orientations (4 rotations × 2 reflections) is easy to under-enumerate — unit-test the full set
  against a hand-drawn reference.

---

## GAME 6 — Cathedral  (suggested title: **"Stadskärnan"** / The Town Core)

**Elevator pitch:** place polyomino "buildings" to claim territory — fully wall off a region with
your color and anything of your opponent's caught inside is captured.

### Rules (Swedish on the rules screen; credit "Baserat på Cathedral" per the naming courtesy above)
- 10×10 board. A single 5-square cross-shaped **Cathedral** piece is placed first (by whichever
  player didn't get first pick, per the usual coin-flip convention), unowned by either side.
  Each player then has a set of secular building pieces (assorted polyominoes, 1–4 squares each,
  reported as **13 pieces per side** — **[verify the exact 13-piece shape roster against a
  rulebook image before implementing; don't guess geometry from memory]**).
- Players alternate placing one piece per turn on empty cells (any orientation/rotation/
  reflection). An empty region that becomes **fully enclosed** — bordered only by one color's
  pieces and/or the board edge... **except** a region touching the board edge is never enclosed —
  is captured: if it contains only the *opponent's* pieces, those pieces are removed and return to
  that opponent's hand to place later; if it contains none of the opponent's pieces, it's simply
  sealed off (no more placements inside it).
- If a player cannot legally place any remaining piece, they pass for the rest of the game.
- **Game end:** neither player can place a remaining piece. **Winner:** the player with the
  **fewest total unplaced squares** left in hand (i.e., whoever got more of their building area
  onto the board).

### game/ model + logic (ink-free, unit-tested)
- Board = 10×10 owner grid (Empty/Black/White/Cathedral). Pieces = a fixed shape table (list of
  relative cell offsets) × 8 orientations.
- `LegalPlacements(b, piece, side)`: every position/orientation where all covered cells are empty
  and in bounds.
- `enclosure(b)`: after each placement, flood-fill every empty region; a region touching the board
  edge is open; otherwise check whether its border cells are entirely one color (+ Cathedral) —
  if so, capture per the rule above (remove opposing pieces to hand; mark friendly-only regions
  sealed so nothing can be placed there again).
- `Winner`: compare remaining hand-square totals when neither side can place.

### AI
- Placement branching (many polyominoes × 8 orientations × up to 100 cells) is large — this is
  the most expensive move generator in the batch. Recommend a **heuristic AI**, not deep
  alpha-beta: maximize (own placed area − risk of being enclosed) each move, with a shallow
  (depth 1–2) lookahead limited to the AI's own top-K candidate placements by that heuristic,
  rather than full-width search. Ship hot-seat as the primary mode; label the AI as a helpful
  practice opponent, not a strong one (same honesty policy as Amazons/Hnefatafl below).

### UI
- 10×10 board; a piece tray below showing remaining pieces per side (tap to select, rotate button,
  tap board to place — only legal placements highlighted).
- **Splash motif:** a small board corner with the cross-shaped Cathedral piece and one small
  building, with a dashed outline suggesting an enclosed region.

### Gotchas
- **Verify the exact 13-piece shape set** before coding — don't ship guessed polyomino geometry.
- Edge-touching regions are never captured — the classic Cathedral bug is capturing a region that
  actually reaches the board boundary.
- Enclosure must be re-checked after **every single placement**, including the opponent's, since
  a region can flip from open to enclosed on either side's move.

---

# BATCH 2 — build after Batch 1

## GAME 7 — Ataxx  (suggested title: keep as-is or **"Smitta"** / Infection)

**Elevator pitch:** clone into an empty neighboring cell, or leap two cells away — either way,
every enemy piece touching your landing spot flips to your color.

### Rules
- 7×7 board (2 corner cells sometimes blocked — optional). Start: 2 pieces each in opposite
  corners. A move is either a **clone** (move distance 1, in any of the 8 directions, original
  piece stays put, new piece appears at the destination) or a **jump** (distance 2, original piece
  vacates its cell). Either way, every orthogonally/diagonally adjacent enemy piece around the
  **destination** flips to the mover's color.
- **Win:** when the board fills or a player has no legal move / no pieces, whoever has more pieces
  wins.

### game/ model + logic
- Board = 7×7 cell grid. `LegalMoves`: clone destinations (distance-1 empty cells) and jump
  destinations (distance-2 empty cells, straight or diagonal). `Apply`: place/move, then flip all
  adjacent enemy cells around the destination.
- `Winner`: board full or side-to-move has no legal move → compare piece counts.

### AI
- Alpha-beta with material + mobility heuristic; moderate branching (up to ~8 clone + up to ~16
  jump destinations per piece) but the board is small (49 cells) — comfortably searchable.

### UI
- Tap a piece → highlight clone cells (near ring) vs jump cells (far ring, visually distinct) →
  tap destination. **Splash motif:** a disc cloning into an adjacent cell with a small flip arrow
  onto a neighboring enemy disc.

### Gotchas
- Clone leaves the source occupied; jump vacates it — a common bug is treating both as a plain
  "move."
- Flip check must scan **all 8** neighbors of the destination, not just 4.

---

## GAME 8 — Konane (keep the name — traditional Hawaiian game)

**Elevator pitch:** a fully-packed checkerboard where **every** move is a jump-capture — no other
move exists. Run out of jumps and you lose.

### Rules
- 8×8 (or 10×10) board, filled alternating black/white stones. Opening: each player removes one of
  their own stones from a fixed starting pair of positions (traditionally a corner and its
  diagonal-adjacent, or the center — the classic opening is Black removes one of the two center
  stones, White removes an orthogonally-adjacent stone next to the gap).
- Every subsequent move is a **jump**: move a stone orthogonally over an adjacent enemy stone into
  the empty cell immediately beyond, removing the jumped stone. A single turn may **chain multiple
  jumps** with the same stone if each is individually legal (player's choice how far to chain —
  not forced to take the longest chain, but must make at least one jump if any exist... actually
  Konane has **no non-jump moves at all**; if a player has zero legal jumps on their turn, they
  lose immediately (no pass).
- **Win:** opponent has no legal jump on their turn.

### game/ model + logic
- Board = 8×8 (or 10×10) grid. `LegalJumpsFrom(b, pos)`: for each of 4 orthogonal directions,
  check enemy-then-empty two cells out; return the landing cell (and support chaining: after one
  jump, recompute further jumps from the new position for the same piece).
- `Winner`: side to move has zero legal first-jumps anywhere on the board.

### AI
- Alpha-beta, material-dominant eval (captures are the only action) + mobility. Moderate
  branching; comfortable depth on an 8×8 board.

### UI
- Tap own stone → highlighted single/chain jump destinations → tap to jump (support tapping again
  to continue a chain, or a "Klart" button to stop early in a chain).
- **Splash motif:** a checkerboard corner with the traditional opening gap and a jump arrow over
  one stone.

### Gotchas
- Chain jumps are optional to extend but the **first** jump each turn is mandatory if any legal
  jump exists anywhere — unit-test "no legal jump anywhere → immediate loss" explicitly.
- The fixed opening-removal step is a one-time special turn, not a normal jump — model it as a
  distinct game phase.

---

## GAME 9 — Isola (keep the name, or suggested **"Vandraren"** / The Wanderer)

**Elevator pitch:** glide your pawn across the board like a queen, then knock out any one tile
behind you. Corner your opponent on a shrinking board.

### Rules
- 8×8 (or 7×7) board, all tiles initially present; each player has 1 pawn on opposite sides. A
  turn = (1) move your pawn any distance in a straight line (any of the 8 queen directions),
  stopping before any missing tile or the opponent's pawn — no jumping over gaps or the opponent —
  then (2) remove any **one** tile from the board (any tile except the one you're standing on).
  **Win:** opponent has no legal move on their turn (no straight-line queen move to any present,
  unoccupied tile).

### game/ model + logic
- Board = grid of present/missing tiles + 2 pawn positions. `LegalMoves`: queen rays truncated at
  the first missing tile or occupied cell (exclusive).
- `Winner`: side to move has zero legal pawn moves.

### AI
- Classic **mobility-difference heuristic** (own legal-move count − opponent's) is the textbook
  strong Isola/Isolation eval — cheap and effective; pair with shallow-to-moderate alpha-beta.

### UI
- Tap a highlighted destination cell (queen-ray, truncated live as tiles vanish), then tap any
  remaining tile (not your own) to remove it. **Splash motif:** two pawns with a scatter of
  removed (checkered/hatched) tiles between them.

### Gotchas
- Removing the tile you're currently leaving (your **old** cell) should be allowed; removing the
  cell you just **moved onto** should not.
- Don't allow "jump over a gap" — line-of-sight must stop dead at the first missing tile.

---

## GAME 10 — The Duke  (suggested title: **"Hertigen"**)

**Elevator pitch:** every tile has a printed move pattern on each face — moving flips it to its
other pattern. Capture the enemy Duke to win; run low on board presence and recruit reinforcements
next to your own Duke.

### Rules (credit "Baserat på The Duke" — Catalyst Game Labs)
- 6×6 board. Each side starts with a Duke tile plus a roster of troop tiles on the board (exact
  starting 6-tile roster and squares — **[verify against the rulebook before implementing; don't
  guess the roster/layout from memory]**); remaining troop tiles start off-board in reserve.
- Each tile is double-sided, with printed relative-offset icons per side: **Move** (slide, blocked
  by intervening pieces), **Jump** (ignores intervening pieces), **Strike** (capture an enemy tile
  at that offset **without** relocating), and combinations. After a tile acts, it **flips to its
  other face** (different pattern next turn) — this includes the Duke.
- **Recruit** (in place of moving): place one reserve troop tile on an empty square orthogonally
  adjacent to your Duke's *current* square — **[verify: does a freshly recruited tile act the
  same turn or only from its next turn — confirm before implementing]**.
- **Win:** capture the opposing Duke.

### game/ model + logic
- Data-driven per-tile-type move table: each (tile type, face) → list of (offset, moveKind) where
  moveKind ∈ {Move, Jump, Strike, MoveOrStrike}. This is the whole engine — implement one generic
  move generator that reads the table, not per-piece special-case code.
- `Apply(move)`: relocate/strike per moveKind, then flip the acting tile to its other face.
- `Winner`: opposing Duke captured.

### AI
- Alpha-beta with material + Duke-safety weighting (heavily penalize Duke-adjacent enemy threats).
  Moderate branching on a 6×6 board — comparable in cost to `shong`.

### UI
- Tap a tile → show its **current face's** legal destinations/strikes (small printed-pattern
  legend accessible from the rules screen, since players won't have memorized ~12 tile faces) →
  tap to act. Recruiting: a "Rekrytera" button while a Duke is selected, then tap an adjacent empty
  square.
- **Splash motif:** a single Duke tile with its move-pattern arrows drawn overlaid, like a legend
  entry.

### Gotchas
- **Verify the exact starting roster/layout and the recruit-tile's same-turn-eligibility rule**
  before implementation — this is the one game in the batch where memory-based rules are least
  trustworthy; don't ship guessed specifics.
- The flip-on-move mechanic means the "legal moves" query must always read the tile's **current**
  face, never a fixed per-type table.

---

## GAME 11 — TZAAR  (suggested title: **"Staplarna"** / The Stacks)

**Elevator pitch:** a hex board, three piece types, no random setup — you place your own army
first. Move a stack exactly as far as it's tall; land on a shorter-or-equal enemy stack to wipe it
out. Lose every piece of any one type and you lose the game.

### Rules (credit "Baserat på TZAAR" — Kris Burm/GIPF project)
- Hex board shared with `ringar`'s geometry (61-cell radius-5 hexagon — reuse the coordinate code).
- Setup phase: players alternate freely placing their own 30 pieces (6 **Tzaar**, 9 **Tzarra**, 15
  **Tott**) onto the empty board, one per turn, in any layout they choose.
- Turn: move one of your stacks in a straight line **exactly N cells**, where N = that stack's
  current height (a lone piece = height 1). Landing on an opposing stack whose height ≤ yours
  captures it entirely (the whole stack is removed). Landing on your **own** stack merges them
  (combined height, capped by nothing — stacks can grow tall). A capturing move is **not**
  mandatory unless it's the only legal move type available that turn — **[verify this "capture not
  forced" detail against the rulebook before implementing; it's the rule most often gotten wrong
  in casual descriptions]**.
- **Win:** the opponent has **zero** pieces of any one type remaining (any one of Tzaar/Tzarra/
  Tott hitting zero ends the game), or the opponent has no legal move.

### game/ model + logic
- Reuse `ringar`'s hex-coordinate + line-ray code. `Stack{Owner, Type, Height}` per occupied cell.
- `LegalMoves(b, side)`: for each own stack of height N, the 6 hex directions, each checked for
  exactly N cells of clear path (no capturing through intervening pieces) landing on empty, own
  (merge), or capturable-enemy (height ≤ N) cells.
- `Winner`: any piece-type count hits zero for either side.

### AI
- Alpha-beta with material (weighted by piece type scarcity — losing your only 6 Tzaars is far
  more urgent than losing Totts) + stack-height/mobility terms. Moderate effort given hex-ray reuse
  from `ringar`.

### UI
- Setup phase: tap an empty hex to drop your next unplaced piece (cycle through remaining
  Tzaar/Tzarra/Tott counts via a small selector). Play phase: tap a stack → highlighted legal
  landing hexes (by exact distance) → tap to move.
- Stack height shown as a small corner numeral on the piece glyph; piece type distinguished by
  shape (reuse `irad`'s △/□/✕-style glyph family: e.g. Tzaar = large circle, Tzarra = square,
  Tott = triangle).
- **Splash motif:** three small hex pieces of different shapes/heights with a capture-distance
  arrow.

### Gotchas
- **Verify "capture not forced"** before coding the AI's legal-move filter — getting this backwards
  changes the whole game.
- Move distance is **exactly** the stack height, never "up to" — unlike a queen's variable range.
- Losing a piece **type** entirely (not total piece count) ends the game — a common
  misimplementation just checks total pieces = 0.

---

## GAME 12 — Amazons (keep the name — academic/public-domain abstract)

**Elevator pitch:** move a queen, then torch a square behind her with an arrow. No captures, ever
— just territory that slowly burns away until someone can't move.

### Rules
- 10×10 board, 4 queens per side at fixed symmetric start squares. Turn: move one of your queens
  any distance in a straight line (rook+bishop directions, no jumping, like a chess queen) to an
  empty square, **then** from that new square shoot an arrow — also a queen-line move — to another
  empty square, which becomes **permanently blocked** for the rest of the game (both queens and
  arrows are stopped by it forever). **Win:** the opponent has no legal queen-move-then-arrow-shot
  available on their turn (last player able to move wins).

### game/ model + logic
- Board = 10×10 grid of {Empty, BurnedArrow, QueenBlack, QueenWhite}. `QueenMoves`/`ArrowShots`:
  standard queen-ray generator, blocked by any occupied/burned cell.
- `Winner`: side to move has no queen with any legal (move, then shoot) pair.

### AI — the honest risk in this batch
- Branching is enormous (up to ~4 queens × ~20 move destinations × ~20 arrow destinations = low
  thousands of full moves per turn at midgame) — **naive deep alpha-beta is not realistic** on this
  hardware. Standard approach used by real Amazons engines: a **territory-flood-fill heuristic**
  (compare, per empty cell, whose queen reaches it in fewer queen-moves — classic "Amazons
  territory count") evaluated at **shallow depth (1, maybe 2)**, not deep search. Ship this and
  label the AI as exploratory/weak — same honesty policy already used for the `goban` Go AI —
  **don't** promise a strong Amazons AI; lean on 2-player hot-seat as the primary mode.

### UI
- Tap a queen → highlighted move destinations → tap one → highlighted arrow destinations from the
  new square → tap to shoot. Burned squares render as a filled/hatched block, distinct from empty.
- **Splash motif:** a queen mid-move with a small arrow icon landing on a hatched "burned" square.

### Gotchas
- Two-phase turn (move, then shoot) needs the same "which action is next" UI clarity called out for
  Quarto's two-phase turn.
- Burned squares block **both** movement and further arrows through them — treat exactly like a
  permanently occupied cell, never a decorative marker.
- Don't over-invest in AI depth — the honest ceiling here is a heuristic opponent, not mastery.

---

## GAME 13 — Hnefatafl  (suggested de-risked v1: **Brandub**, 7×7; full spec: Copenhagen 11×11)

**Elevator pitch:** an asymmetric Viking siege — a king and his defenders try to break out to any
corner, while a much larger attacking force tries to surround and capture him first.

### De-risk like Six was de-risked: build **Brandub** first (7×7, 8 attackers vs. 4 defenders +
king), not the full 11×11 Copenhagen ruleset — same board/piece engine, far less content and
board-legibility risk; upgrade to 11×11 later only if Brandub proves out well.

### Rules (Copenhagen ruleset, for the full version; Brandub is the same rules on a 7×7/8-attacker
layout — credit "Baserat på Hnefatafl" is unnecessary, this is a public-domain traditional game)
- 11×11 board (Brandub: 7×7). 5 special squares: the center **throne** and the 4 **corners**. Only
  the king may ever stop on the throne or a corner; other pieces may not land there (whether or not
  they may pass through an *empty* throne is a common house-rule choice — simplest and safest: treat
  the throne as impassable terrain for all non-king pieces, like a permanent wall, rather than
  merely unstoppable-on).
- All pieces move like a rook: any distance orthogonally, no jumping, blocked by other pieces and
  the restricted squares (per above).
- **Custodial capture** (regular pieces): sandwich one-or-more enemy pieces between two of your own
  along a straight line — same mechanic as `hasami`. The **empty** throne counts as a hostile
  square for **both** sides when computing a sandwich (a commonly-misremembered detail — it is not
  attacker-only).
- **King capture:** 4 attackers on all orthogonal sides when the king is on an open square; only 3
  attackers needed if the king is adjacent to the (empty) throne, since the throne itself supplies
  the 4th hostile side; 4 attackers still required if the king is actually on the throne.
- **Win:** defenders win when the king reaches **any corner square**. Attackers win by capturing the
  king, or if the defenders have no legal move. Defenders also win if the attackers have no legal
  move (rare but possible).

### game/ model + logic
- Reuse `hasami`'s custodial-capture scan almost verbatim, generalized for: (a) the empty throne as
  a hostile square for both sides, (b) the king's separate 3-or-4-sided capture rule, (c) corner
  escape as a distinct win check each turn.
- Two different `Winner` checks depending on side (asymmetric — don't share one generic "reduce to
  N pieces" check like Hasami's).

### AI
- Needs **two distinct evaluation functions** — attackers value king-distance-to-nearest-corner
  and containment/encirclement; defenders value the king's escape-route count and open lines to a
  corner. This asymmetry is the real engineering cost, more than search depth. Alpha-beta is fine
  computationally (board is small even at 11×11); the eval design is the work. Build and tune
  against Brandub's smaller board first.

### UI
- Distinguish attacker/defender/king by shape (not just fill), since this is 3 piece roles, not 2 —
  e.g., attacker = filled disc, defender = outline ring, king = outline ring with a small crown
  glyph. Mark throne/corners with a distinct cell pattern.
- **Splash motif:** a king piece on a center throne glyph, flanked by two attacker discs closing in.

### Gotchas
- Empty-throne-hostile-to-both-sides is the single most commonly wrong detail in fan
  implementations — unit-test it explicitly for both attacker and defender captures.
- King's capture-square-count (3 vs. 4) depends on adjacency to the throne — don't hardcode "always
  4."
- Build Brandub (7×7) first; don't start with the full 11×11 Copenhagen ruleset.

---

# BATCH 3 — hidden-info / economy games

These four round out the "modern eurogame" side of the library alongside `sushi` (currently the
only card/drafting game). **Pick at minimum Hanamikoji or Lost Cities** if effort needs trimming —
both are 2-player, small, and iconic; Splendor and Patchwork are the higher-value of the remaining
"one-pick-per-niche" alternates already named in the skip list (Jaipur/Century overlap Splendor's
economy-engine niche; Kingdomino/Schotten Totten overlap Patchwork's/Lost Cities' niches
respectively).

Copy `sushi/`'s drafting-AI scaffolding (a greedy expected-value heuristic with light lookahead,
since these are all hidden-information games where plain minimax doesn't apply) as the AI template
for all four, not `hasami`/`othello`'s perfect-info alpha-beta.

## GAME 14 — Hanamikoji  (suggested title: **"Geishorna"**, credit "Baserat på Hanamikoji")

**Elevator pitch:** tiny ruleset, rich tactical squeeze — split, offer, and hide cards to win the
favor of more geishas than your opponent.

### Rules — **[the 3 non-"Secret" action mechanics below are flagged for a rulebook double-check
before implementation; the Secret action and the overall charm-majority win condition are solid]**
- 7 geishas with charm values {2, 2, 2, 3, 3, 4, 5} (sum 21); 21 item cards distributed across the
  7 geishas (count per geisha matches — **[verify exact per-geisha card counts]**). Each round,
  deal a hand of cards to each player (one card is set aside face-down/"burned", unseen this
  round).
- Each player has 4 action markers, one use each per round (order of your own choosing), 8 total
  action-turns per round, alternating:
  1. **Secret**: place 1 card face-down; added to your collection, hidden from the opponent until
     round-end reveal.
  2. **Trade-off**: place 3 cards face-up; opponent keeps 1 of the 3 for their collection, the
     other 2 go to you — **[verify: does the *acting* player or the opponent choose which 1?]**.
  3. **Gift**: split your remaining cards into a group of 2 and a group of 1; opponent picks which
     group you receive, opponent keeps the other — **[verify group sizes and who picks]**.
  4. **Competition**: split into two groups of 2; opponent assigns one group to you and keeps the
     other — **[verify]**.
- **Round end** (once both players have used all 4 actions): reveal Secret cards; for each geisha,
  whoever has more item cards of that geisha's type wins that geisha's charm (ties favor neither).
  **Round win:** whoever controls geishas totaling ≥ 11 of the 21 charm points, **or** controls ≥ 4
  of the 7 geishas (either condition ends the round immediately in that player's favor).
- **Match win:** first player to win 2 rounds.

### game/ model + logic
- `Geisha{Value int}` × 7; `Card{Geisha int}` × 21. Round state: each player's collection
  (revealed + hidden-until-reveal), remaining hand, 4 unused action flags.
- Each action is an independent, unit-testable function operating on (actingPlayer, chosen cards)
  → updates both collections per the exact split/choice rules above.
- `RoundWinner`: charm-majority-per-geisha tally → ≥11 points or ≥4 geishas.

### AI
- Hidden-info — no plain minimax. Greedy expected-value heuristic per action (mirror `sushi`'s
  drafting AI): when it's the AI's turn to choose a split/keep, evaluate by charm-value-weighted
  card counts it can infer from public information (revealed cards + its own hand), not full
  opponent-hand knowledge.

### UI
- Small, dense card-and-track UI: 7 geisha tracks show accumulated public cards; hand shown as
  tappable cards; each action has its own tap flow (Trade-off: tap 3 cards to offer; Gift/
  Competition: tap to assign cards into groups, present groups to the opponent/AI to choose from).
- **Splash motif:** two geisha-track icons with a couple of item cards leaning toward one side.

### Gotchas
- **This is the one game in the whole batch most likely to have an implementation-affecting rules
  error from memory alone** — verify the Trade-off/Gift/Competition group sizes and "who chooses"
  against an official rules PDF before writing the action functions.
- Tie on a geisha's charm (equal card counts) awards it to **neither** player — easy to forget.

---

## GAME 15 — Lost Cities  (suggested title: **"Expeditionen"**, credit "Baserat på Lost Cities")

**Elevator pitch:** hand management with one meaningful decision every turn — commit cards to an
expedition in ascending order, or cut your losses and discard.

### Rules
- 5 suits/expeditions, each with 9 number cards (2–10) and 3 "investment"/wager cards — 12 cards
  per suit, 60 total. Turn: play one card to your own expedition row (must be ≥ the last card
  you played in that suit; **all** investment cards for a suit must be played before any number
  card in that suit) **or** discard a card face-up onto that suit's discard pile — then draw 1 card
  (from the deck, or the top of any suit's discard pile).
- **Scoring per suit** (after the deck runs out): an untouched expedition scores 0. Otherwise:
  `(sum of number cards played − 20) × (1 + investment cards played in that suit)`, then **+20**
  flat bonus if **8 or more** cards were played in that expedition. Total score = sum across all 5
  suits.
- **Win:** higher total score. (Base game is a single round; can extend to a match of N rounds.)

### game/ model + logic
- `Card{Suit, Rank}` where Rank ∈ {Investment×3, 2..10}; deck of 60. Per-player expedition rows
  (last-played rank per suit, investment count per suit) + 5 discard piles.
- `LegalPlay(card, expeditionRow)`: investments only before any number card; number cards strictly
  ascending vs. the row's current top.
- `Score(expeditionRow)`: the exact formula above, including the ×(1+investments) multiplier
  applying to the whole `(sum − 20)`, not just the number-card sum alone, and the +20 applied
  **after** the multiplier.

### AI
- Hidden-info (opponent's hand is secret; discard piles are public). Heuristic: value each
  candidate play by expected-marginal-score vs. "is this suit worth entering at all" (a suit
  needs sum ≥ 20 just to break even, more with investments) — mirror `sushi`'s expected-value
  drafting AI approach rather than any minimax.

### UI
- 5 expedition rows + 5 discard piles laid out as a simple grid; tap a hand card, tap "Spela" onto
  its suit row or "Kasta" onto its discard pile, then tap a draw source (deck or a discard top).
- **Splash motif:** one expedition row showing an ascending run of 3–4 cards with a small score
  readout.

### Gotchas
- The **investments-before-numbers** ordering constraint per suit is the classic rule newcomers
  miss — unit-test that a number card can't be played into a suit's row before at least matching
  the suit's already-played investment requirement... more precisely: once **any** number card is
  played in a suit, no further investment cards may be added to it. Test both directions.
- Scoring formula operator precedence — `(sum − 20) × multiplier`, then `+ 20` bonus — is easy to
  reorder incorrectly; unit-test against a couple of known hand-worked examples.

---

## GAME 16 — Splendor  (suggested title: **"Juvelerna"** / The Gems, credit "Baserat på Splendor")

**Elevator pitch:** collect gem tokens, buy development cards that become permanent discounts, and
race to 15 prestige — a clean perfect-information engine-builder, no hidden hands at all.

### Rules
- 5 gem-token colors (count scales by player count: 4 each for 2-player, this app's target — plus
  5 gold/wildcard tokens always). 90 development cards across 3 tiers (40/30/20), each costing some
  combination of colored gems and granting a permanent 1-gem discount of its own color plus some
  prestige points. **Nobles**: (players + 1) drawn face-up — 3 for a 2-player game — each worth 3
  prestige, auto-claimed at the end of a turn if that player's accumulated card-bonuses meet the
  noble's requirement (if multiple qualify at once, the active player picks one).
- On your turn, do exactly one of: take 3 tokens of 3 different colors; take 2 tokens of the same
  color (only legal if that color's pile had ≥ 4 before taking); reserve a face-up or face-down
  card (max 3 reserved at once) and take 1 gold token if any remain; or purchase a card (face-up
  from the tableau, or from your own reserve) paying with tokens and/or your permanent card
  discounts (gold substitutes for any color). Hand size cap: 10 tokens total — discard down if
  over, at end of turn.
- **Win:** first player to reach **≥15 prestige** at the end of their turn triggers game-end; finish
  the current round (so all players get equal turns) then highest prestige wins, ties broken by
  fewest development cards owned.

### game/ model + logic
- Straightforward perfect-info state: token bank, per-player tokens/cards/reserved-cards/nobles,
  tableau (3 tiers × 4 face-up slots + draw piles). Each action (take-3, take-2, reserve, buy) is
  an independent legality-checked function.
- `NobleCheck`, `EndTrigger`, `FinalWinner` (round-completion + tiebreak) as separate, testable
  functions.

### AI
- Perfect information — a straightforward greedy-plus-shallow-lookahead heuristic (value each
  action by progress toward affordable tier-appropriate cards + noble requirements) is enough for
  a fun opponent; a full alpha-beta is possible too since the branching per turn is modest, but not
  necessary to get a decent AI here.

### UI
- The busiest layout in this batch (tableau of 12 cards across 3 tiers + gem bank + player's own
  cards/tokens/reserves + noble row) — **keep it 2-player only**, same UI-density mitigation
  `mosaik` already used for Azul: show the active player's board full-size, opponent as a compact
  summary/toggle.
- **Splash motif:** a small stack of 2–3 gem tokens next to a development-card outline showing a
  prestige-point corner.

### Gotchas
- The take-2-same-color action requires the pile to have had **≥4 before** taking, not ≥2 — an
  easy off-by-one.
- Multiple nobles qualifying simultaneously is the active player's choice, not automatic/first-
  match.
- Game-end is "finish the round," not "stop immediately at 15 prestige" — matters for 2-player
  turn parity too.

---

## GAME 17 — Patchwork  (suggested title: **"Lapptäcket"** / The Quilt, credit "Baserat på
Patchwork")

**Elevator pitch:** a 2-player race to tile your 9×9 quilt board efficiently, spending buttons and
time to buy patches off a shared circular track — pure spatial optimization, no luck once a game
starts (the patch order is public/shared).

### Rules
- A circular track of ~33 patches (varied polyomino shapes, each with a button-cost, a time-cost,
  and a button-income value) plus a **neutral token** that also sits on the track. Each player has
  a 9×9 quilt board (starts empty) and a personal marker on the *linear* time track (0–53).
- **Turn order:** whichever player's time-track marker is **furthest behind** always acts next (if
  tied, a fixed tiebreak — commonly "the player earlier in turn order," i.e. player 1); that player
  either (a) buys one of the **3 patches nearest clockwise** to the neutral token — pay its button
  cost, place it anywhere (any rotation/reflection) fully on their empty quilt cells, advance the
  neutral token past the bought patch, then advance their own time-marker by the patch's time-cost
  — or (b) if they can't/won't buy, advance their own time-marker to just past the trailing
  opponent's position, gaining 1 button per square advanced this way.
- **Income:** the time track has marked "button income" squares; landing on **or passing** one
  pays out buttons equal to your quilt board's total button-income value (sum of placed patches'
  income stats).
- **Special 1×1 patches:** ~8 single-square scoring tiles sit at fixed time-track positions; the
  first player whose marker reaches/passes that position takes it for free and places it on their
  board (each is worth 1 permanent button-income).
- **7×7 bonus:** the first player to completely fill any 7×7 area of their board with patches
  scores an immediate +7 bonus (claimable once, by whichever player gets there first).
- **Game end:** when both players' markers have reached the end of the time track (53).
  **Final score:** `buttons in hand − 2 × (empty squares remaining on your 9×9 board)`, plus the
  +7 bonus if you earned it.

### game/ model + logic
- Track = ordered list of patch slots + neutral-token position + each player's time-marker.
  `NextActor`: trailing-marker rule. `BuyPatch`/`Advance` as the two turn actions.
- Board = 9×9 bool grid per player; `CanPlace(shape, orientation, pos)` = fits entirely within
  empty cells. Income trigger = crossing threshold detection on the linear time track (careful with
  "passing," not just "landing exactly on").
- `SevenBySeven(b) bool`: scan for any fully-filled 7×7 window — flag/reuse-once.
- `FinalScore`: the formula above.

### AI
- Perfect information (nothing hidden — the whole track and both boards are visible) — this is
  actually the **cleanest AI-friendly game in Batch 3** despite the economy theme. A greedy
  heuristic (buy the patch maximizing income-per-button-spent while avoiding board fragmentation
  that can't fit later shapes) with shallow lookahead is enough; deeper alpha-beta is possible
  given full information but not required for a fun opponent.

### UI
- The trickiest UI in the batch: circular track rendering + two 9×9 boards + shape-placement with
  rotation. Keep the circular track abstracted as a simple horizontal strip (a straight scrolling
  row is far easier to render/tap on e-ink than an actual circle) showing the next several patches;
  full-size board for the active player, opponent board as a compact summary — same 2-player
  density mitigation as Splendor/Azul above.
- **Splash motif:** a small 3×3 quilt-board corner with 2–3 differently-shaped patches placed.

### Gotchas
- "Passing" an income/bonus square must be detected via the marker's start→end range that turn, not
  just its final resting cell — moving several spaces at once is normal here.
- The trailing-player-acts-next rule (not strict alternation) is the single most important turn-
  order detail to get right; unit-test a few interleaved-turn sequences explicitly.
- The 7×7 bonus is claimable **once total**, not once per player — first to achieve it gets it,
  and it's gone for the rest of the game.

---

## Recommended build order

1. **Chomp**, **Domineering**, **L Game** — trivial engines, near-perfect/perfect AI, cheapest wins
   in the whole doc; good warm-up/parallelizable batch.
2. **Breakthrough** — cheap, genuinely strong AI, no surprises.
3. **Quoridor** — the flagship pick; medium effort, AI needs the heuristic/candidate-pruning
   approach spelled out above, not naive alpha-beta.
4. **Cathedral** — novel enclosure mechanic; verify the 13-piece shape roster before coding.
5. **Ataxx**, **Konane**, **Isola** — same tier of effort as Batch 1's larger entries, round out
   the "small perfect-info abstract" shelf.
6. **The Duke**, **TZAAR** — verify their flagged rules details first, then build; both reuse
   existing engines (`shong`'s AI shape; `ringar`'s hex geometry for TZAAR).
7. **Hnefatafl** — build **Brandub (7×7)** first, not the full Copenhagen 11×11.
8. **Amazons** — build last among the perfect-info abstracts; the AI is an honest weak spot, ship
   hot-seat as primary and label the AI as exploratory.
9. **Hanamikoji**, **Lost Cities** — first hidden-info picks from Batch 3, both small/iconic;
   verify Hanamikoji's action-mechanics details first.
10. **Splendor**, **Patchwork** — round out Batch 3; Patchwork's circular-track UI is the fiddliest
    rendering task in this whole document, budget real on-device layout testing.

Every build still follows the standard §0 setup + splash/rules screens + `play_test.go` from
`SPEC_NEXT_GAMES.md`/`SPEC_BUILD_GAMES.md` and the guide.
