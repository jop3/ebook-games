# Build spec — recommended next games

Full hand-off specs for the six games graded **✅ Build** in `GAME_FEASIBILITY_EVAL.md`, plus a
small Othello variant-mode appendix. Build order (low-risk first):

1. **Hasami** (Hasami shogi) — forks `othello`, strong AI is cheap
2. **Shong** — tiny 4×6 chess-like duel; strong AI trivial
3. **Munkar** (Donuts) — direction-forcing placement + custodial capture
4. **Sushi** (Sushi Go) — the library's first card/drafting game
5. **Ringar** (YINSH) — deep abstract; ring/marker flipping
6. **Go** — iconic; 2-player primary, weak 9×9 AI optional

Every game follows the **same non-negotiable setup + splash/rules requirements as
`SPEC_NEXT_GAMES.md` §0 and `POCKETBOOK_GAMEDEV_GUIDE.md`** — read those first. The per-game
sections below only add what's specific to each game.

---

## 0. READ FIRST (applies to all six — same as SPEC_NEXT_GAMES §0)

- **Read `POCKETBOOK_GAMEDEV_GUIDE.md` in full.** Toolchain, the `ink` API, the traps.
- Target: PocketBook Verse Pro (PB634), **1072×1448 (usable height 1340), greyscale, tap-only**,
  32-bit ARM.
- **Per-game boilerplate** (copy an existing module and gut it):
  - New Go module `<game>/` with vendored `third_party/inkview` + `go.mod` replace.
  - **Copy `othello/`** for the four board games with an AI (Hasami, Shong, Munkar, Ringar, Go);
    **copy `othello/` or a fresh module** for Sushi.
  - Keep ALL rules/AI logic in `game/` (no `ink` import) → unit-tests cgo-free via `check.ps1`.
  - Input: `Pointer` on `PointerUp`, `Touch` on `TouchUp` fallback → one `handleTap(p)`.
  - Fonts opened ONCE in `Init()` into a `*Fonts` struct; never `OpenFont` in `Draw`.
  - `ScreenSize()` only inside `Draw`/`Init`, never the constructor; seed 1072×1448.
  - End `Init()` with `ink.Repaint()`.
  - Redraw full each `Draw`; `FullUpdate()` on state change / every ~6–8 frames, else `PartialUpdate`.
- **MANDATORY splash + rules screens** (guide §10): `screenSplash` is the initial state; menu with
  difficulty/mode + a **"Regler"** button → `screenRules`; **full rules in Swedish**; per-game
  splash motif. Hardware Back returns to menu.
- **Verify + ship** (guide §6/§7/§8): unit-test logic; render every screen at worst case in the
  emulator; `play_test.go` (`//go:build playtest`, `TestPlay…`) driving a full game via real taps;
  ARM `.app` under a clean filename; 8-bit BMP icon (+`_f`); register in `view.json` @Games.
- **ASCII-safe app titles** (guide §5a/§8) — no å/ä/ö in the launcher title; Swedish is fine in
  rules-screen body text (the device renders it correctly).

### Naming / attribution (three of these are published commercial games)
Rules are not copyrightable, but **names, art, and themes are**. Shong (free, Higher Plain Games),
Donuts (Funforge 2021), Sushi Go (Gamewright), and YINSH (Kris Burm / Rio Grande) — reimplement the
*rules* with **original greyscale art** and the **neutral Swedish titles** above (the library
already does this: "Einsteins Gata", "Ordskrav"). Go and Hasami shogi are public-domain traditional
games. Credit the originals in the rules screen ("Baserat på …") as a courtesy.

### AI scaffolding (shared by the five 2-player games)
All five reuse `othello`'s pattern: compute the AI move **after painting the human's move** (an
`aiPend` flag consumed on the next `Draw`, so the board updates before the device "thinks"), and a
pure `BestMove(state, side, difficulty) Move` in `game/`. Alpha-beta minimax with a per-game
evaluation and a depth/'`Lätt/Medel/Svår`' knob. Board sizes below are small enough that this is
cheap — no MCTS anywhere except the optional Go AI.

---

## GAME 1 — Hasami (Hasami shogi)  ⭐ build first

**Elevator pitch:** move your men like rooks and **capture by sandwiching** the enemy between two
of your pieces. Reduce your opponent to a single piece to win.

### Rules (Swedish on the rules screen)
- 9×9 board. Each player has **9 men** filling their nearest rank (Black on the bottom row, White
  on the top row).
- A man moves like a **rook**: any distance straight horizontally or vertically, no jumping, onto
  an empty square.
- **Custodial capture:** if, *as a result of your move*, a straight orthogonal line of one or more
  enemy men is bounded at **both** ends by your men, all those enemy men are captured (removed).
  A single move can trigger captures in several directions at once.
- **Corner capture:** an enemy man in a board corner is captured when you occupy the **two**
  cells orthogonally adjacent to that corner.
- **Safe entry:** moving your own man *into* the gap between two enemy men is **not** self-capture —
  capture only ever happens on the mover's turn, caused by the mover. Unit-test this.
- **Win:** reduce the opponent to **1 man** (default "Fångst" mode).
- **Optional mode "Fem i rad":** first to make an unbroken line of **5** of their men (horizontal
  or vertical) anywhere **outside their own starting rank** wins. (Same 9-piece/rook setup — a
  house-rule toggle, not the 18-piece Dai variant.)

### game/ model + logic (ink-free, unit-tested)
- `type Cell uint8` = `Empty|Black|White`; `type Board [9][9]Cell`; `Side`.
- `LegalMoves(b, side) []Move` — `Move{From, To image.Point}`; rook rays from each own man.
- `Apply(b, m) (Board, []image.Point)` — returns the new board and the captured cells; capture
  resolution runs the custodial + corner scan from `m.To` for the mover's side only.
- `Winner(b, mode) Side` — Fångst: a side with ≤1 man loses; Fem-i-rad: scan for a 5-line outside
  the owner's home rank.
- Capture scan (the core to get right): for each of the 4 directions from `m.To`, walk over a
  contiguous run of enemy men; if it terminates in a friendly man (and the run length ≥1), capture
  the run. Corner rule handled as a special case on the four corner cells.

### AI
- `BestMove(b, side, diff)`: alpha-beta minimax. Eval = **material diff** (dominant) + mobility +
  a small center/advancement term + "men one move from being captured" penalty. Depth Lätt 2 /
  Medel 3 / Svår 4. Branching is moderate (≤ ~9 men × rook rays); depth 4 is comfortable on device.
- Add win/loss shortcuts (immediate capture that wins; avoid moves that hang a decisive capture),
  mirroring `othello`'s `BestMove`.

### UI
- 9×9 grid centered; men = solid black disc / outline white ring (reuse `othello`'s disc/ring).
- Tap own man → highlight legal destinations → tap a destination. Briefly mark captured cells
  before clearing (a fading X or an inverted flash on `PartialUpdate`).
- Menu: mode (Fångst / Fem i rad), opponent (2 spelare / Mot dator + Lätt/Medel/Svår), "Regler".
- Buttons in-game: "Ny", "Meny". Status line: whose turn / "Svart vann" etc.
- **Splash motif:** a short horizontal line — white ring flanked by two black discs (`● ○ ●`) with
  a capture arrow — instantly conveys the sandwich rule.

### Gotchas
- **Safe entry** (moving between two enemies is safe) is the classic rule newcomers get wrong —
  unit-test it explicitly, both orthogonal and at a corner.
- A move can capture in **multiple directions**; resolve all four before checking `Winner`.
- Corner capture is a separate code path from line capture — test each of the 4 corners.
- Fem-i-rad must exclude the owner's **home rank** (else the start position trivially counts).

### Definition of done
- [ ] `game/` unit tests: legal rook moves; single/multi-direction custodial capture; corner
      capture; safe-entry non-capture; both win modes vs an independent checker.
- [ ] AI takes a winning capture and avoids hanging a decisive one; full game vs AI in `play_test.go`.
- [ ] Splash + rules (Swedish) + menu (mode/opponent/Regler); all screens clean in emulator.
- [ ] ARM `.app` `hasami.app`, icon + `_f`, view.json @Games.

---

## GAME 2 — Shong  ⭐ tiny board, strong AI for free

**Elevator pitch:** a lightning chess-like duel on a **4×6** board. Pieces alternate between short
and long moves; trap the enemy King or run your own King to the far edge.

*Baserat på Shong (Higher Plain Games) — free abstract; reimplement with original art.*

### Rules (Swedish on the rules screen)
- Board **4 columns × 6 rows**. Four piece types:
  - **Triangel** — moves diagonally.
  - **Kvadrat** — moves orthogonally (vertical/horizontal).
  - **X** — moves in any of the 8 directions (like a queen's directions).
  - **Kung** — moves exactly **one** step, alternating each turn between the Triangel move-set
    (diagonal) and the Kvadrat move-set (orthogonal).
- **Short/long toggle:** Triangel/Kvadrat/X start on a **short move (1 square)**. After a piece
  makes its first move it is marked (an "eye"/dot) and its subsequent moves are **long (exactly 2
  squares)**, then it stays long. (Model as a per-piece `moved bool`; short = 1, long = 2.)
- **No jumping** — every move needs a clear line of sight; a blocked path is illegal. Landing on an
  enemy piece captures it (standard displacement capture).
- **Win:** (a) **capture the enemy King**, or (b) move **your King to the opponent's back row**
  (the far edge).

**Starting setup (our default — confirm against official rules if exact fidelity is wanted):**
each player gets **4 pieces on their back rank**, mirrored: columns 0–3 =
`X, Triangel, Kvadrat, Kung`. Black on row 0, White on row 5. This is a balanced, quick-conflict
layout on the 4-wide board; the movement rules above are the authoritative part.

### game/ model + logic (ink-free, unit-tested)
- `type Kind uint8` = `Triangle|Square|Ex|King`; `type Piece struct{ Kind; Side; Moved bool }`;
  `type Board [6][4]*Piece` (nil = empty).
- `stepSet(kind)` → the direction vectors (diagonal / orthogonal / all-8); King uses diagonal on
  even plies-of-its-own, orthogonal on odd (track `kingOrtho bool` per side, flips each King move).
- `LegalMoves(b, side)`: for each own piece, ray in its directions for the exact distance (1 if
  `!Moved` else 2; King always 1), requiring the path clear and the landing square empty-or-enemy.
- `Apply(b, m)`: move piece, set `Moved=true` (flip the King's parity), capture on landing.
- `Winner(b)`: a side whose King is captured loses; a King on the far edge wins for its owner.

### AI
- `BestMove` alpha-beta. **The board is only 24 cells** → search deep cheaply (Svår depth 6–8 is
  fine; the original ships three AI levels). Eval = King safety + King's distance-to-goal (race!) +
  material + mobility. This is the one game on the list where a genuinely **strong** AI is easy —
  make Svår actually hard.

### UI
- 4×6 grid, large cells (narrow board leaves generous width). Pieces drawn as their literal
  glyphs — **Triangel △, Kvadrat □, X, Kung** (a crown/■-with-notch) — reuse `irad`'s proven
  △ □ X mark-drawing. A small dot/eye on a piece marks its long-move state.
- Tap own piece → highlight legal squares → tap to move. Show the King's current move-mode (diag
  vs orth) as a tiny indicator.
- Menu: 2 spelare / Mot dator (Lätt/Medel/Svår) + "Regler". Buttons "Ny", "Meny".
- **Splash motif:** the four pieces △ □ X ♚ in a row (mirrors the chess-app splash), one with an
  "eye" dot to hint the short/long mechanic.

### Gotchas
- The **exact-distance** move (long = 2, not "up to 2") is unusual — a long piece cannot stop at 1.
  Unit-test that a long move of 1 square is illegal and the path's middle square must be clear.
- The King's **alternating** move-set is per-King state — test that it flips only when the King
  actually moves, not every ply.
- Two win conditions — test both the King-capture and the reach-far-edge endings.
- Reconstruct/confirm the starting layout early; it's the one under-documented detail.

### Definition of done
- [ ] `game/` unit tests: each piece's short and long moves; blocked-path illegality; King
      alternation; displacement capture; both win conditions.
- [ ] AI: Svår beats a random/greedy opponent convincingly; full games in `play_test.go` (both
      win types reached).
- [ ] Splash + rules (Swedish) + menu; emulator-clean.
- [ ] ARM `.app` `shong.app`, icon + `_f`, view.json @Games.

---

## GAME 3 — Munkar (Donuts)

**Elevator pitch:** each square points a direction; the ring you place **dictates where your
opponent must play next**. Flank enemy rings to flip them, and line up 5 of your color to win.

*Baserat på Donuts (Funforge) — reimplement with original art + this neutral name.*

### Rules (Swedish on the rules screen)
- **6×6** board built from four **3×3** tiles. Every cell shows a **line orientation**: horizontal
  `—`, vertical `│`, or diagonal `╱` / `╲`. (We define our own four tiles — see below — since the
  art is original anyway.)
- Players alternate placing a ring of their color on an empty cell (first player places anywhere).
- **Direction-forcing:** the line on the cell you just filled dictates the **line through that cell**
  (its row / column / diagonal) along which your **opponent must place their next ring**. If every
  cell on that line is already occupied, the opponent may place **anywhere**.
- **Capture (custodial, on placement):** after you place a ring, look along each of the 4 axes
  (H, V, ╱, ╲) through it. If a **contiguous run of your rings** (including the new one) is bounded
  **immediately at both ends by an opponent ring** — pattern `E Y…Y E` — both bounding opponent
  rings **flip to your color**. (This is the rulebook's `O_O` and `OXX_O` examples: filling the gap
  so your rings sit between two enemies converts those two enemies.) Up to 2 flips per axis.
- **Win:** immediately on **5 of your rings in a line** (row, column, or diagonal). If the board
  fills with no 5-line, the player with the **largest orthogonally-connected group** of their rings
  wins (tie → draw, or last-placer, pick one and state it).

**Our four fixed 3×3 tiles (design-locked — any balanced mix works):** distribute the 36 cells so
each orientation appears roughly equally and no line is monotonous, e.g. each tile a rotation of:
```
— │ ╱
╲ — │
│ ╱ —
```
Shuffle the four tiles' positions/rotations at new-game (that's the only board randomness).

### game/ model + logic (ink-free, unit-tested)
- `type Orient uint8` = `H|V|D1|D2`; `board.Line [6][6]Orient` (fixed per game);
  `board.Ring [6][6]int8` = `0 empty / 1 / 2` (player).
- `ForcedCells(b, last image.Point) []image.Point`: the empty cells on `last`'s line (per its
  `Orient`) through `last`; empty slice ⇒ opponent unconstrained.
- `Place(b, p, player) (Board, []image.Point flips)`: set the ring, then run the custodial scan on
  all 4 axes and flip the bounding enemy pairs; return flips for the UI.
- `Five(b, player) bool`; `LargestGroup(b, player) int` (orthogonal flood-fill). `Winner`.

### AI
- `BestMove` alpha-beta over legal placements (respect the forced-direction constraint in move
  generation). Eval = own longest-line potential − opponent's + captures gained − exposure to being
  captured + group size. 6×6 with a shrinking empty set → shallow-but-fine depth (Medel 3, Svår 4).

### UI
- 6×6 grid; each cell faintly shows its line glyph (`— │ ╱ ╲`) as light-grey line-art; rings drawn
  as **filled disc (you)** vs **outline ring (opponent)** — greyscale-clean.
- After a placement, **highlight the forced line** for the next player (thin band) so the constraint
  is obvious; flash flipped rings.
- Tap an empty (and, when constrained, legal) cell to place; illegal taps rejected with a hint.
- Menu: 2 spelare / Mot dator (Lätt/Medel/Svår) + "Regler". Buttons "Ny", "Meny".
- **Splash motif:** a few cells with line glyphs and a `● ○ ●`-style flank showing a flip.

### Gotchas
- The **capture rule is inverted from Othello** (your run bounded by enemies flips the *enemy*
  bookends, not an enemy run bounded by you) — unit-test `O_O→OXO⇒XXX` and `OXX_O→OXXXO⇒XXXXX`
  along every axis, and confirm a placement with no bounding enemy flips nothing.
- The **forced-direction** move generator is the fiddly input rule — test the "line full ⇒ play
  anywhere" fallback and that diagonals are handled for both `╱` and `╲`.
- Endgame tiebreak (largest group) must be spelled out on the rules screen and matched by `Winner`.

### Definition of done
- [ ] `game/` unit tests: forced-direction generation (incl. full-line fallback); custodial flips
      on all 4 axes for both example patterns; 5-in-a-line win; largest-group tiebreak.
- [ ] AI respects the forced constraint and plays a full legal game in `play_test.go`.
- [ ] Splash + rules (Swedish) + menu; emulator-clean (line glyphs legible at cell size).
- [ ] ARM `.app` `munkar.app`, icon + `_f`, view.json @Games.

---

## GAME 4 — Sushi (Sushi Go)  ⭐ the library's first card game

**Elevator pitch:** a fast card-drafting game — take one card, pass the rest, repeat; collect sets
of sushi for points over three rounds.

*Baserat på Sushi Go! (Gamewright) — reimplement with original icons + this neutral name.*

### Rules (Swedish on the rules screen)
- **1 human vs 2–4 AI** (hidden simultaneous hands make hot-seat awkward — go human-vs-AI).
- Deal a hand to each player (hand size by player count, e.g. 5p→7, 4p→7, 3p→8, 2p→10). Each turn,
  **everyone simultaneously picks one card** to keep, then **passes the rest of their hand** to the
  next player. Repeat until hands are empty. Play **3 rounds**; keep **Pudding** across rounds.
- **Scoring** (score after each round except Pudding):
  - **Nigiri:** Ägg 1 / Lax 2 / Bläckfisk 3.
  - **Wasabi:** triples the value of the **next Nigiri** you play (Ägg 3 / Lax 6 / Bläckfisk 9);
    must be played before its nigiri.
  - **Tempura:** each **pair** = 5 (odd one leftover scores 0).
  - **Sashimi:** each **set of 3** = 10 (incomplete sets score 0).
  - **Dumpling (Dumplings):** 1/2/3/4/5+ cards = **1/3/6/10/15**.
  - **Maki-rullar:** most maki icons this round = **6**, second-most = **3** (shared on ties, split
    down; in 2p drop the 2nd-place award or keep per your chosen ruleset — state it).
  - **Chopsticks (Ätpinnar):** on a later turn, swap it back into the hand to **take 2 cards**
    instead of 1.
  - **Pudding (end of game only):** most = **+6**, fewest = **−6** (shared; in 2p no −6).
- **Win:** most points after 3 rounds + pudding.

### game/ model + logic (ink-free, unit-tested)
- `type CardKind uint8` (the ~9 kinds above, with maki 1/2/3-icon and nigiri egg/salmon/squid as
  data). `Hand []Card`; `Tableau` per player (played cards).
- `Deck` composition (fixed counts per the game). Deal, draft, pass rotation.
- `ScoreRound(tableaus) []int` — one **pure function per scoring category**, summed (each category
  independently unit-tested — the same discipline as the puzzle validators). `ScorePudding`.
- Drafting turn engine: collect one pick per player, apply chopsticks (a player holding chopsticks
  may pick 2 and return the token), rotate hands, detect round end.

### AI
- Drafting heuristic (no deep search needed): expected-value per card given the player's tableau and
  what's likely to come around — e.g. value the 3rd sashimi highly if you already hold 2, chase
  maki majority only if contested, take wasabi before nigiri, grab pudding when behind on pudding.
  A greedy EV with 1-ply "will it wheel back?" estimate plays a fine opponent. Three strengths =
  add noise/greediness knobs.

### UI (this one is NOT a board — design fresh, but reuse fonts/buttons)
- Top: your **tableau** (cards you've kept this round) grouped by kind with counts + running score.
- Middle: your **current hand** as a row/grid of tappable cards; tap a card to draft it. If holding
  chopsticks, a "Använd ätpinnar" button enables a 2-card pick.
- Bottom: opponents' visible tableaus (compact), round/turn indicator, "Meny"/"Regler".
- **Card art = ~9 simple greyscale icons** (nigiri, maki-roll, tempura, sashimi, dumpling, wasabi,
  pudding, chopsticks) — legible at card size; the nigiri/maki subtypes shown by pips/number. This
  is the main art task; prototype icons in the emulator early (guide §0a: real TTF text renders).
- Round-end and game-end **score screens** (breakdown by category), then next round / final banner.
- **Splash motif:** three distinct sushi icons in a row (nigiri, maki, dumpling).

### Gotchas
- Simultaneous drafting: resolve all picks **then** pass — don't let one player's pick affect
  another's same-turn choice. Unit-test the pass rotation and chopsticks 2-pick.
- Each scoring category has edge cases (leftover tempura, incomplete sashimi, maki ties, 2p pudding)
  — one unit test per category vs a hand-computed expected score.
- Pudding persists across rounds; everything else resets. Don't reset pudding.
- Hidden information: never render opponents' hands, only their played tableaus.

### Definition of done
- [ ] `game/` unit tests: deck composition; draft/pass/chopsticks engine; **every** scoring
      category + pudding vs independent hand-scored expectations; a full 3-round game total.
- [ ] AI drafts legally and reaches a sensible score; `play_test.go` plays a full 3-round game
      through the real card-tap UI and asserts the final ranking.
- [ ] Splash + rules (Swedish) + menu (player count 2–5, difficulty, Regler) + round/end score
      screens; all emulator-clean; icons legible.
- [ ] ARM `.app` `sushi.app`, icon + `_f`, view.json @Games.

---

## GAME 5 — Ringar (YINSH)

**Elevator pitch:** move rings to flip a trail of markers Othello-style; make a row of five of your
color to remove a ring — remove three of your rings and you win (getting weaker as you near
victory).

*Baserat på YINSH (Kris Burm, GIPF-projektet) — reimplement with original art + this neutral name.*

### Rules (Swedish on the rules screen)
- Board = a hexagonal field of **85 intersections** on a triangular grid (a truncated six-point
  star). Lines run along **3 axes**.
- **Placement phase:** starting with White, players alternate placing their **5 rings** on empty
  intersections. (10 placements total.)
- **Movement:** choose one of your rings; drop a **marker of your color** on its current point, then
  slide the ring in a straight line along an axis to an empty point. The ring may pass over **any
  number of markers** but must **stop on the first empty point after the last marker jumped**, and
  may **not** pass over or land on another ring. Every marker jumped is then **flipped** (color
  reversed) — exactly like an Othello flip along the moved segment.
- **Rows:** a **row of 5 adjacent markers of one color** along any axis (formed on *either*
  player's turn) lets **that color's owner** remove those 5 markers **and one of their own rings**
  (the ring is taken off the board as a scoring token).
- **Win:** the first player to have removed **3 of their own rings** wins. (If a move creates rows
  for both players, the mover resolves theirs first — state the tie handling on the rules screen.)

### game/ model + logic (ink-free, unit-tested) — the hardest logic of the six
- **Coordinate system:** use axial/cube hex coords masked to the 85 valid points (precompute the
  point set + the 3 axis direction vectors). This is the one real research bit — nail it with a
  unit test enumerating exactly 85 points and correct neighbours.
- `type Point`; `rings map[Point]Side`; `markers map[Point]Side`.
- `RingMoves(state, from) []Point`: slide along each of 6 ray directions (3 axes × 2), applying the
  "jump markers, stop after the last one, blocked by rings" rule.
- `ApplyRingMove(state, from, to)`: place marker at `from`, move ring, flip every marker strictly
  between `from` and `to`.
- `FindRows(state, side) [][]Point`: all length-≥5 monochrome straight runs (report each 5-window).
- `RemoveRow` + `RemoveRing`. `Winner(state)` = side with 3 rings removed.

### AI
- `BestMove` alpha-beta with a **modest depth (2–3)** + strong eval, because YINSH's branching is
  large: eval = rings-removed diff (dominant) + count/length of your near-complete rows − opponent's
  + marker majority + ring mobility. Ship this as the "Mot dator" opponent and be honest in the menu
  that the AI is a **casual** opponent; hot-seat 2-player is the primary experience. (A stronger AI
  is a future upgrade, not a launch requirement — same stance as the Hex MCTS idea in the guide.)

### UI
- Reuse **`hex`'s triangular/hex board rendering** for the point grid + line-drawing. Rings =
  **outline circles**; markers = **solid disc (black)** vs **outline/white disc** — a literal
  black/white game, ideal for e-ink.
- Two-tap move: tap a ring (highlight its legal destinations along the 3 axes), tap a destination;
  animate nothing — just redraw with the marker dropped, ring moved, and flips applied (a
  `FullUpdate` to clear ghosting after flips).
- When a row of 5 forms, **highlight it** and prompt the owner to pick which ring to remove (tap a
  ring). Show each side's removed-ring count (0/3) prominently.
- Placement phase has its own tap flow (tap empty point to place a ring until all 10 down).
- Menu: 2 spelare / Mot dator (casual) + "Regler". Buttons "Ny", "Meny".
- **Splash motif:** a 7-hex honeycomb (reuse hex's flower) with 2 rings and a short trail of
  black/white markers, one row highlighted.

### Gotchas
- The **85-point geometry + 3 axes** is the make-or-break; build and unit-test the board model
  before any UI.
- Ring-move rule: **must stop on the first empty point after the last jumped marker** — not "any
  empty point." Test jumps over 0, 1, and several markers, and blocking by rings.
- Flips apply to **exactly the jumped markers** (strictly between from/to), then row-detection runs.
- A single move can complete rows for **both** players and can complete **multiple** rows — define
  and test the resolution order.
- Removing a ring is the *scoring* action and reduces your future mobility — make the "which ring to
  remove" choice explicit, not automatic.

### Definition of done
- [ ] `game/` unit tests: exactly-85-point board + neighbours; placement; ring-move jump/stop/block;
      marker flips; 5-row detection (incl. >5 and double rows); ring removal; 3-rings win.
- [ ] AI plays full legal games vs itself/human in `play_test.go`; no illegal moves, win reached.
- [ ] Splash + rules (Swedish) + menu; emulator-clean at the full 85-point board.
- [ ] ARM `.app` `ringar.app`, icon + `_f`, view.json @Games.

---

## GAME 6 — Go

**Elevator pitch:** the classic. Place black and white stones to surround territory and capture
enemy groups. **The best possible greyscale game** — the stones literally *are* black and white.

### Rules (Swedish on the rules screen)
- Sizes **9×9 / 13×13 / 19×19** (default 9×9 — best for the device and the AI). Players alternate
  placing a stone on an empty intersection; Black first.
- **Capture:** a group with no liberties (no empty adjacent points) is removed. You may not play a
  **suicide** move (self-capture with no resulting capture).
- **Ko:** you may not immediately recreate the exact previous board position (simple positional/ko
  rule) — track the prior position and forbid the repeat.
- **Passing / end:** two consecutive passes end the game.
- **Scoring — area (Chinese):** your score = your stones on the board + empty points your stones
  fully surround; add **komi** (default 6.5 to White on 9×9) to White. Highest score wins.

### game/ model + logic (ink-free, unit-tested)
- `type Color uint8`=`Empty|Black|White`; `type Board [][]Color` (size-parameterised).
- `Group(b, p)` (flood-fill same color) + `Liberties`. `Place(b, p, c)` → remove captured enemy
  groups first, then reject if the played group now has no liberties (suicide). `Ko` check against
  the previous board hash.
- `Legal(b, p, c, koPrev) bool`; `AreaScore(b, deadSet, komi) (blk, wht float64)`.
- **Dead-stone handling at end (the pragmatic, correct-for-casual approach):** after two passes,
  enter a **mark-dead** phase — players tap groups to toggle them dead; removed dead stones count as
  the surrounder's area. This sidesteps automatic life/death (which is genuinely hard) and is the
  standard casual-app solution.

### AI (optional — do NOT over-invest)
- **Primary experience is 2-player hot-seat.** For "Mot dator", ship a **weak 9×9 opponent**:
  either a light heuristic (capture/atari-aware, avoid self-atari, play near contested groups,
  prefer big empty areas) or a very shallow Monte-Carlo playout (a few hundred random rollouts per
  candidate) if it profiles fast enough on device. **Label it "svag" (weak).** A strong Go AI is out
  of scope for this ARM chip — don't chase it; 9×9 only for the AI.

### UI
- Line grid with star points (hoshi); stones = **solid black disc** / **solid white disc with a
  thin outline** — perfect greyscale, no translation needed. Tap an intersection to place; show the
  last move with a small marker and captured stones vanishing on the redraw.
- Buttons: **"Passa"**, "Ny", "Meny"; a stone/prisoner + score readout. On two passes → mark-dead
  phase (tap groups) → final score screen.
- Menu: size (9/13/19), opponent (2 spelare / Mot dator (svag), 9×9 only), komi, "Regler".
- **Splash motif:** a small grid corner with a black group in atari (one liberty) about to be
  captured by white — teaches liberties at a glance.

### Gotchas
- **Capture-before-suicide order:** remove opponent groups reduced to zero liberties *before*
  testing the played stone for suicide (a move that captures is legal even if the stone would
  otherwise be self-atari). Unit-test capturing and the suicide rejection separately.
- **Ko:** forbid the immediate board-repeat; test the classic ko shape.
- Scoring: area (Chinese) is simpler and more robust to dead-stone marking than territory
  (Japanese) — stick with it. Test seki/neutral points don't over-count.
- e-ink: big captures change many cells → `FullUpdate` after captures to clear ghosting.

### Definition of done
- [ ] `game/` unit tests: liberties/capture; suicide rejection; ko; area scoring with a marked
      dead set + komi; two-pass end.
- [ ] 2-player full game + (if shipped) a weak-AI 9×9 game in `play_test.go`; capturing, ko, and
      final scoring exercised.
- [ ] Splash + rules (Swedish) + menu (size/opponent/komi/Regler) + mark-dead + score screens;
      emulator-clean at 19×19 worst case.
- [ ] ARM `.app` `go.app` (or `goban.app` to avoid the toolchain name), icon + `_f`, view.json @Games.

---

## Appendix — Anti-Othello variant mode (not a new app)

The recommended substitute for a standalone "Desdemona" (see `GAME_FEASIBILITY_EVAL.md`): add a
**mode toggle** to the existing `othello` app.

- **Rule:** identical Othello play; at the end the player with the **fewest** discs wins ("Omvänd
  Othello" / Anti-Othello — a recognised Reversi variant).
- **Effort:** ~a one-line win-condition flip + a menu toggle + AI sign-flip (the eval negates, so
  `BestMove` maximises *losing*). No new module, icon, art, or view.json entry.
- **Done when:** the menu offers "Vanlig / Omvänd", the win banner and AI both respect the mode, and
  a `play_test.go` case plays an Anti-Othello game to the reversed win.

---

## Cross-game build order & shared checklist
Build in the numbered order (risk-ascending). For **every** game, the definition-of-done above plus
the universal gates from `SPEC_NEXT_GAMES.md` §"Definition of done": pure `game/` logic with tests,
splash+rules (Swedish), menu with mode/difficulty + "Regler", all screens emulator-verified at worst
case, ARM `.app` under a clean name with an 8-bit icon (+`_f`) registered in `view.json` @Games,
`*_render_test.go` removed, and the guide's device app list bumped.
