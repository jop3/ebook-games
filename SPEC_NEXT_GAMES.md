# Build spec — next games: Akari, Slitherlink, Quarto (+ group 4)

Hand-off spec for the next session(s). **Priority batch (build first, in this order):**
**1) Akari (Light Up), 2) Slitherlink, 3) Quarto!**

**Group 4 (build after the priority batch — full specs also included below):**
**4) Hashiwokakero, 5) Kakuro, 6) Nurikabe.** These are solid but overlap cognitively with
puzzles we already have a lot of (constraint/number grids), so they come second.

They fill gaps in our "golden library of 20": we currently have 11/20 (Mastermind,
Bulls & Cows, Black Box, Einstein/logic-grid, Nonogram, Othello, Hex, Connect Four via
"I rad", Reversi=Othello, Sudoku). The priority batch takes us to 14/20; group 4 to 17/20.
After that only Tak, Onitama, Sprouts remain (see end).

**All six games follow the same setup + splash/rules requirements in §0.**

---

## 0. READ FIRST (non-negotiable setup)

- **Read `C:\github\Ny mapp\POCKETBOOK_GAMEDEV_GUIDE.md` in full before writing code.** It has the
  toolchain, the ink API, and every trap. This spec assumes it.
- Also skim memory: `pocketbook-toolchain`, `pocketbook-input-gotcha` (esp. Pointer-not-Touch,
  fonts-once, filename cache, view.json recipe, ScreenSize-not-in-constructor).
- Target: PocketBook Verse Pro (PB634), **1072×1448 portrait, greyscale, tap-only**, 32-bit ARM.

### Per-game boilerplate (identical to existing games — copy an existing module as template)
- New Go module `C:\github\Ny mapp\<game>\` with vendored `third_party/inkview` + go.mod replace.
  **Fastest: copy `nonogram/` (for Akari/Slitherlink) or `othello/` (for Quarto) and gut it.**
- Keep ALL rules/solver logic in `game/` (no `ink` import) so it unit-tests cgo-free via
  `scratchpad/check.ps1 <game>`. UI/drawing in `ui.go` (+ `main.go` implements `ink.App`).
- **Input:** handle taps in `Pointer` on `PointerUp`, `Touch` on `TouchUp` as fallback, both →
  one `handleTap(p image.Point)`. Dispatch by screen.
- **Fonts:** open every (typeface,size) ONCE in `Init()` into a `*Fonts` struct; reuse via
  `SetActive`. Never `OpenFont` inside `Draw`.
- **ScreenSize:** query inside `Draw()` (or `Init()`), NEVER in the constructor (returns garbage
  pre-`Run`). Seed structs with 1072×1448 default.
- **Repaint in Init():** end `Init()` with `ink.Repaint()` (else the first tap is dead).
- **Redraw cadence:** full-screen redraw each Draw; `FullUpdate()` on state change / every ~6–8
  frames to clear ghosting, else `PartialUpdate`.

### MANDATORY for every new game: splash + rules screens (guide §10)
Both are standard now — **all 14 existing games have them, these must too.** Follow guide §10
verbatim. Summary:
- `screen` enum with `screenSplash` as the FIRST/initial state, then `screenMenu`, the game
  screen(s), and `screenRules`. `Init()` sets screen = `screenSplash`.
- `DrawSplash(screen, fonts, title, motif)`: big bold title at `Y/6`, a centered square motif box
  (`side = X*3/5`) drawn by a per-game `motifFunc`, grey hint "Tryck för att börja" at `Y*5/6`.
  Tap anywhere (or Back) → menu.
- `DrawRules(screen, fonts, title, paragraphs) image.Rectangle`: bold title, word-wrapped body
  paragraphs (`wrapText`/`splitWords` helpers, body ~34px, margin 60, lineH 46, paraGap 24),
  centered "Tillbaka" button (return its rect). A "Regler" button on the menu opens it. Back key
  from rules → menu. **Write the FULL rules in Swedish** (these are single-player logic games —
  clear rules matter).
- Motif = simple monochrome line-art of the game's own pieces (see each game below).

### Verify + ship (guide §6, §7, §8)
- Unit-test the solver/generator logic with `check.ps1 <game>` (cgo-free).
- Render every screen (menu, splash, rules, game mid-solve, solved) to PNG with the inkrender
  emulator via `scratchpad/render.ps1 <game> <Test> SPLASH_OUT=<dir>`. **The emulator now shows
  REAL text (TTF)** — verify titles, clue numbers, and layout at the WORST case (largest grid).
- Build ARM `.app` via the Docker SDK image (guide §7). Confirm `ELF 32-bit ARM`.
- Deploy: the device uses **clean filenames** (`akari.app`, no version suffix) matching the
  `U_<name>` key in view.json, absolute paths, string-form icons (guide §8). Generate 8-bit BMP
  icons (`scratchpad/mkicon/`) + `_f` focused variant. Add each to view.json @Games via
  `scratchpad/vjfinal/main.go`. Device re-reads view.json on USB-disconnect (no reboot).
- Delete any `*_render_test.go` before shipping.

### Cross-cutting lessons from PocketPuzzles source (verified against actual `games/*.c`)

Studied the real C source of SteffenBauer/PocketPuzzles (an official-SDK C port of Simon Tatham's
puzzle collection to this exact device class — see guide §12 for the general survey). These
patterns recur across ≥3 of their games and apply to every generator/solver we write below, not
just one game — check each per-game "Gotchas" section too for game-specific findings.

1. **"Reject if solvable one tier down" is their universal difficulty-grading rule.** Every one of
   their generated-puzzle games (Bridges, Loopy, Light Up, Solo, Towers) re-solves the finished
   puzzle at `target_difficulty - 1` and discards/regenerates if it ALSO succeeds there — i.e. a
   puzzle must require exactly the claimed tier, not just "be solvable at or below it." Apply this
   check to every solver/generator pair in this spec (Akari, Slitherlink, Kakuro, Nurikabe, Towers,
   Singles), not only where explicitly mentioned below.
2. **Generation is retry-by-full-restart, not incremental repair**, almost everywhere. Their outer
   generation loops are `while(1)`/`goto generate` that throw away a failed attempt entirely and
   start over, bounded only by informal limits — a minimum grid-size floor, a small numeric retry
   cap (their range: 20–1000 depending on how expensive one attempt is), or in a couple of cases
   nothing but an acknowledging code comment. **None of them implement a hard wall-clock timeout**
   in the solver/generator itself. Match this shape (bounded retry + full restart + a fallback that
   always lets the app start), but since we don't have their years of tuning, err toward adding an
   explicit iteration cap even where their source shows none — safer on unfamiliar hardware.
3. **Grid-size ceilings were hand-tuned per game based on real on-device generation-speed limits,
   not screen space** — e.g. their Towers is capped at 9×9 with a source comment admitting "the
   solver just isn't fast enough" beyond that, while their Singles (cheaper solver) goes to 12×12.
   Lesson: don't assume a uniform "keep it ≤9×9" rule across all our games — the right ceiling
   depends on how expensive that specific game's solver is per cell, so budget real gen-time
   testing per game rather than copying one game's size cap onto another.
4. **A shared "armed digit, tap-to-stamp" numeric entry UI** (tap a digit button to arm it, then
   tap grid cells to stamp that digit repeatedly, no per-cell popup) is their proven cross-game
   standard for tap-only numeric entry (used identically in Sudoku and Towers). Worth adopting for
   any game here needing digit entry (Kakuro, Towers) as an alternative/upgrade to "select cell,
   then tap keypad" if that flow feels slow in on-device testing.
5. **Never rely on color to distinguish state** — confirmed directly: their README states color is
   entirely substituted with greyscale/texture on this device, and their changelogs show repeated
   contrast-tuning passes (bold font for generator-given clues vs. player input, shaded error
   backgrounds instead of colored ones). We already know this (guide's greyscale-only note) — this
   is independent confirmation from a mature, shipped codebase on the same hardware class.

Where a specific game's Gotchas section below cites PocketPuzzles source directly with concrete
numbers (retry caps, size ceilings), treat those as the more authoritative, game-specific version
of the general rules above.

---

## GAME 1 — Akari (Light Up)   ⭐ build first (easiest, great e-ink fit)

**Elevator pitch:** illuminate every white cell of a grid by placing light bulbs, respecting the
numbered walls. Pure deduction, no guessing.

### Rules (put a Swedish version on the rules screen)
- Grid of cells. Some are **black wall cells**, the rest white. Some walls carry a number 0–4.
- Player places **bulbs** in white cells. A bulb lights its own cell and shines in the four
  orthogonal directions until a wall (or grid edge) blocks it.
- **Win when:** every white cell is lit, AND no bulb shines onto another bulb (two bulbs may not
  share a row/column segment with no wall between them).
- **Numbered wall:** exactly that many bulbs must be orthogonally adjacent to it. `0` = none.

### game/ model + logic (ink-free, unit-tested)
- `Cell`: Wall(number -1..4) | White. `Board`: W×H of cells; player state = set of bulbs.
- `lit(board, bulbs)`: compute the lit set by ray-casting each bulb 4 directions until a wall.
- `Valid`/`Solved` checks: all white lit; no bulb lights another; every numbered wall's adjacent
  bulb count matches.
- **Solver (for unique generation):** deduction propagation is enough for fair puzzles —
  forced-bulb rules (a white cell no bulb can ever light must contain one; a `4` wall forces all
  4 neighbours; a cell that would leave a wall unsatisfiable is barred). Implement a solver that
  returns Unique / Stuck / Contradiction (mirror nonogram's `LineSolvable` pattern).
- **Generator:** place random walls + a random valid bulb layout → derive wall numbers → strip to
  a puzzle → accept only if the solver yields a unique solution by pure logic. Retry loop with a
  cap + fallback (same shape as `nonogram/game/state.go` `Generate`). Sizes: e.g. 7×7, 10×10,
  14×14 (Lätt/Medel/Svår). Keep gen < ~50 ms.

### UI
- Reuse a nonogram-style grid layout (cell = fit to screen; center; button bar at bottom).
- Tap a white cell to cycle: empty → bulb → (optional) dot-mark "known empty" → empty.
- Draw: wall = filled black cell (number centered in white if present); bulb = filled disc /
  sun glyph; lit cells = light-grey wash; a rule-violation (two bulbs seeing each other) = mark
  both with an X so the player sees the conflict.
- Buttons: "Rensa" (clear), "Meny". On solve: status "Löst!" + "Nytt pussel".
- **Splash motif:** a small grid with 2–3 bulbs and their light rays (grey lines) crossing a wall.
- Difficulty menu: Lätt 7×7 / Medel 10×10 / Svår 14×14 + a "Regler" button.

### Gotchas
- Ray-casting must stop AT the wall (wall cell itself not lit by that ray).
- The "no bulb lights another" rule is symmetric — check each bulb's rays for another bulb.
- **From PocketPuzzles' `lightup.c` source:** their generator retries **20** inner attempts (place
  walls with symmetry, greedily fill bulbs, derive numbers, test solvability); if all 20 fail it
  widens the search by bumping the wall percentage +5 and loops again — a good concrete fallback
  strategy if a fixed wall-density retry loop keeps failing to converge. They also enforce the
  universal difficulty discipline seen across all their games: **reject a generated puzzle if it's
  ALSO solvable one difficulty tier down** (re-solve at `target-1`; discard and regenerate if it
  succeeds) — apply this to our solver/generator contract too, not just "solvable at target tier."
  Their hardest tier allows a small bounded **recursive guess depth capped at 5** — if our pure
  logic-only solver above ever proves too weak to generate enough puzzles, a shallow bounded-guess
  fallback (depth ≤5) is a proven, cheap escape hatch rather than full backtracking.
- Their tap UI uses **two independent toggle actions** (short tap = bulb, long-press = "known
  empty" X-mark) that each reject if the opposite mark is already on that cell — cleaner than a
  single 3-state cycle-on-tap if long-press is easy to detect reliably on our device; worth
  considering as an alternative to the cycle described above if testing shows cycle-taps feel slow.

---

## GAME 2 — Slitherlink   ⭐ most satisfying logic puzzle

**Elevator pitch:** draw a single closed loop on a dot grid so each numbered cell is bordered by
exactly that many loop edges.

### Rules (Swedish on rules screen)
- Grid of dots forming W×H cells. Some cells have a clue 0–3.
- Player toggles **edges** (segments between adjacent dots) on/off.
- **Win when:** the ON edges form **exactly one single closed loop** (no branches, no crossings,
  no separate loops), AND every clue equals the number of ON edges around that cell.
- Common helper: let the player also mark an edge as "definitely off" (an ✕ on the segment).

### game/ model + logic (ink-free, unit-tested)
- Dots at (x,y) for x in 0..W, y in 0..H. Edges: horizontal (between (x,y)-(x+1,y)) and vertical.
  Represent edge state as two boolean grids (hEdges, vEdges) or a map.
- `clueSatisfied`: for each numbered cell, count its 4 surrounding edges that are ON.
- `isSingleLoop(edges)`: **the hard part.** Every dot must have degree 0 or 2 (never 1 or 3+);
  the ON edges must form exactly one cycle covering all ON edges (use union-find over dots +
  a total-degree/one-component check, or walk the loop and confirm it returns to start and
  consumes every ON edge). Return true only for a single closed loop.
- **Solver (unique generation):** slitherlink is line/edge-solvable with the standard local rules
  (corner/edge deductions from 0s and 3s, degree ≤2 per dot, no premature small loops). Implement
  enough propagation to certify uniqueness, or a bounded backtracking solver that counts
  solutions and aborts at 2 (mirror einstein's "0/1/>1, early-abort-at-2" approach).
- **Generator:** generate a random single loop on the grid (e.g. random Hamiltonian-ish loop or
  grow a loop), derive each cell's edge count as its clue, then strip clues while the solver still
  proves a unique solution. Accept only unique. Sizes: 5×5, 7×7, 10×10.

### UI
- Draw dots as a grid; clue numbers centered in cells; ON edges as thick black segments; OFF-marks
  as small ✕ on the segment midpoint.
- **Tap targets are the EDGE midpoints, not cells.** Compute a hit rect around each h/v segment
  midpoint; tap cycles ON → ✕(off) → blank. Make hit rects generous (segments are thin).
- Buttons: "Rensa", "Meny"; on solve "Löst!" + "Nytt pussel".
- **Splash motif:** a few dots with a short closed loop drawn between them + a clue number or two.
- Difficulty menu 5×5 / 7×7 / 10×10 + "Regler".

### Gotchas
- Edge hit-testing is the trickiest input in this batch — test it in the emulator with injected
  taps at segment midpoints. Give each segment a fat tappable band.
- `isSingleLoop` must reject the "all clues satisfied but two separate loops" case — that's the
  classic bug. Unit-test it explicitly.
- **From PocketPuzzles' `loopy.c` source:** their generator builds a FULL solved loop first
  (`add_full_clues`), checks it's uniquely solvable, THEN digs holes (`remove_clues`) shuffling
  clue-removal order and rolling back any removal that breaks uniqueness — i.e. solution-first,
  same shape already planned above, good confirmation. Their own comment admits the full-clue
  generation loop **"can loop for ever if the params are suitably unfavourable"** and their only
  real defense is refusing grid sizes below 4×4 — worth adding an explicit iteration cap +
  fallback ourselves rather than trusting "small grids are safe" alone, since we can't easily
  verify their exact threshold transfers to our solver's performance.
- They use **4 difficulty tiers gated by named solver passes**, tried cheapest-first each
  iteration (trivial/loop-closure deductions → dot-pair reasoning → harder equivalence-class
  reasoning) — mirrors the same "try easy deductions before harder ones, loop until stuck"
  structure our solver should use for both correctness and speed.
- They deliberately **pruned their on-device grid-size presets smaller than desktop** ("remove
  grid types that don't really make sense on limited screen size") — reinforces keeping our sizes
  modest (5×5/7×7/10×10 as planned) rather than assuming bigger is fine just because it fits.
- No fixed pixel-radius constant for edge hit-testing was found in their source (they delegate to
  a generic nearest-edge geometry function) — so there's no magic "correct" radius to copy; budget
  real on-device tap testing as planned above, don't assume a formula will get it right first try.

---

## GAME 3 — Quarto!   ⭐ the most distinctive addition (2-player + AI)

**Elevator pitch:** 4-in-a-row where all pieces are shared and **you choose which piece your
opponent must place**.

### Rules (Swedish on rules screen)
- 16 unique pieces, each with 4 binary attributes: tall/short, light/dark, round/square,
  hollow/solid. 4×4 board.
- Turn structure: on your turn you **place the piece your opponent handed you** anywhere empty,
  then **choose a piece from the remaining pool and hand it to your opponent**.
- **Win:** complete a line (row, column, or main diagonal) of 4 pieces that **all share at least
  one attribute**. (Classic base game: lines only. Optional 2×2-square variant — skip for v1.)
- The player who places the winning piece wins (even though the opponent chose it for them).

### game/ model + logic (ink-free, unit-tested)
- Piece = 4-bit value 0..15 (bit per attribute). Board = 16 cells (piece index or empty).
  Pool = set of unused pieces.
- `winningLine(board)`: for each of the 4 rows, 4 cols, 2 diagonals, if all 4 filled and
  `AND of pieces != 0 OR (NOT each) AND != 0` → i.e. they share a 1-bit or share a 0-bit. Use
  bitmask: shareOne = `p0&p1&p2&p3 != 0`; shareZero = `^p0&^p1&^p2&^p3 & 0xF != 0`. Win if either.
- AI (for vs-device mode): **minimax with alpha-beta over BOTH decisions** (which cell + which
  piece to give). State is small enough late-game; early-game cap depth (e.g. depth 3–4) or use
  the known heuristic (never hand a piece that lets the opponent complete a shared line). Provide
  win/avoid-loss shortcuts like othello's `BestMove`.

### UI (copy othello's board + menu structure)
- 4×4 board, big cells. **Rendering the 4 attributes in greyscale is the design challenge** —
  make each attribute a clear visual, e.g.: shape = square vs circle; size = big vs small;
  fill = solid vs ring/hollow; "hole" = a white dot in the center vs none. So a piece is e.g.
  "big solid square with a hole". Draw a small legend on the menu or rules screen.
- Below the board: the **pool** of remaining pieces (tappable), and a prominent slot showing
  **"piece you must place"** (the one handed to you).
- Turn flow UI: (1) a piece is highlighted as "to place" → tap an empty board cell to place it;
  (2) then the pool becomes active → tap a piece to hand it over; turn passes. Show whose turn
  and, in AI mode, "Datorn väljer…" while it thinks (compute AFTER painting, like othello's
  `aiPend`).
- Modes: 2 spelare (hot-seat) / Mot dator. Menu + "Regler".
- **Splash motif:** four distinct Quarto pieces in a row (show the attribute contrasts:
  big-solid-square, small-ring-circle, etc.) — mirrors the chess splash's 4 pieces nicely.

### Gotchas
- The win test's "share a 0-bit" half is easy to forget — a line of four SHORT pieces wins even
  though `p0&p1&p2&p3 == 0`. Unit-test both share-one and share-zero wins.
- Two-phase turn (place, then give) needs clear UI state so the player knows which action is next.
- Attribute glyphs must be unambiguous on e-ink at cell size — prototype in the emulator early.

---

# GROUP 4 — build after the priority batch

All three are single-player, generated, uniquely-logic-solvable puzzles — same family as
Nonogram/Sudoku/Einstein, so **copy `sudoku/` or `nonogram/` as the template** and reuse the
generate-and-verify pattern (generator emits only puzzles the solver proves have exactly one
logic solution). Same §0 setup + splash + rules requirements.

---

## GAME 4 — Hashiwokakero ("Broar" / Bridges)

**Elevator pitch:** connect all the numbered islands with bridges so every island has exactly its
number of bridges and the whole network is one connected piece.

### Rules (Swedish on rules screen)
- Grid with **islands** (circles) each holding a number 1–8. The rest is water.
- Player draws **bridges** = straight horizontal/vertical lines between two islands in the same
  row/column with only water between them.
- Constraints: **at most 2 bridges** between the same pair of islands; bridges are straight and
  **never cross** each other; each island's total bridge count must equal its number; and **all
  islands must end up connected** into a single group.
- **Win:** every island's number satisfied AND the whole set is connected.

### game/ model + logic (ink-free, unit-tested)
- `Island{X,Y,Need int}`; puzzle = list of islands (+ grid dims). Player state = bridge count
  (0/1/2) per ordered island pair that are orthogonal neighbours-with-clear-line.
- Precompute, for each island, its up/down/left/right **neighbour island** (first island along
  that direction with only water between). Bridges may only exist between such neighbour pairs.
- `crosses(a,b, c,d)`: a horizontal bridge and a vertical bridge cross if their spans intersect —
  forbid adding a bridge that would cross an existing one.
- `degree(island)` = sum of its bridge counts; `Solved` = all degrees == Need AND connectivity
  (union-find over islands via active bridges = one component).
- **Solver (unique gen):** Hashi is deduction-solvable with local rules — e.g. an island whose
  Need forces max bridges on all neighbours (a `4` with two neighbours ⇒ 2+2), islands where
  remaining capacity forces a bridge, "an isolated pair can't take the last bridge if it would
  close the network early". Implement propagation returning Unique/Stuck/Contradiction
  (mirror nonogram `SolveResult`), or a bounded solution-counter aborting at 2 (einstein pattern).
- **Generator:** place islands, build a random valid bridge layout that's fully connected, derive
  Need from the layout, then present islands+needs as the puzzle; accept only if the solver proves
  it unique. Sizes: e.g. 7×7 (~8 islands), 10×10, 13×13. Cap gen time.

### UI
- Islands as circles with the number centered; a selected island highlighted.
- **Input:** tap island A then island B (a neighbour) to cycle the bridge between them 0→1→2→0.
  (Simpler + more reliable than dragging.) Show a hint of tappable neighbours when one island is
  selected. Draw single bridge = one line, double = two parallel lines.
- Colour a satisfied island (degree==Need) differently (e.g. filled) so progress is visible.
- Buttons "Rensa", "Meny"; on solve "Löst!" + "Nytt pussel". Menu: 3 sizes + "Regler".
- **Splash motif:** 3–4 islands with a couple of single/double bridges between them.

### Gotchas
- Crossing detection is the classic bug — a new bridge must not cross ANY existing perpendicular
  bridge. Unit-test it.
- Connectivity is part of the WIN condition, not just the per-island counts — a layout can satisfy
  every number yet be two separate rings. Test that case.
- Two-tap input needs clear "island A selected, pick a neighbour" state feedback.
- **From PocketPuzzles' `bridges.c` source:** their generator does NOT build a solution then strip
  clues (unlike most of their other games) — it **grows the island layout directly** via randomized
  graph expansion (pick a random existing island, extend in a random open direction by a random
  distance, or join an existing island), retrying a bad placement up to **50** times before giving
  up and accepting a smaller layout. Confirms our island-placement generator (§ above) is a
  reasonable, independently-validated approach — don't feel obliged to switch to a "full solution
  first" pattern for this game specifically, it's the odd one out among the six.
  Also confirms the difficulty discipline: solvability is checked AFTER generation via a
  from-scratch solve, and (if difficulty > lowest tier) it also verifies the puzzle is NOT solvable
  one tier down — reject/restart otherwise. Apply the same "must need exactly this tier" check.
  Their island-vs-neighbour bridge UI is drag-based (island→island drag, long-press to remove) —
  we're deliberately choosing the simpler two-tap select-then-cycle model instead (per the UI
  section above), which is a reasonable divergence given our device has no fast drag support.
  One eInk-specific note from their changelog: they had to **explicitly reduce full-screen redraws**
  for Bridges after real on-device performance problems — reinforces our own §5/§6 guidance
  (Partial vs Full update) to redraw only what changed on each bridge toggle, not the whole board.

---

## GAME 5 — Kakuro ("Sifferkorsord")

**Elevator pitch:** a crossword where every "word" is a sum — fill digits 1–9 so each run adds up
to its clue, with no repeated digit in a run.

### Rules (Swedish on rules screen)
- Grid of black clue-cells and white entry-cells. A clue-cell may carry a **down-sum** (top-right
  triangle) and/or a **right-sum** (bottom-left triangle).
- Each horizontal **run** (white cells right of a right-sum) must sum to that clue; each vertical
  run (below a down-sum) must sum to its clue. **Digits 1–9 only, no digit repeats within a run.**
- **Win:** every run sums correctly with no in-run repeats and all white cells filled.

### game/ model + logic (ink-free, unit-tested) — closest to `sudoku/`
- Cell types: Block (with optional down/right clue) | Entry(value 1–9 or empty).
- Parse the grid into **runs**: each run = ordered list of entry cells + its target sum.
- `runOK(run)`: filled cells have no repeats and (if complete) sum == target; partial run must not
  already exceed target or repeat.
- **Solver:** constraint propagation with candidate bitmasks per cell (1..9) + backtracking — reuse
  `sudoku/game/solver.go` structure. Strong pruning via subset-sum: for a run of length L summing
  to S, only certain digit combinations are possible (precompute allowed-digit masks per (L,S)).
- **Generator:** fill a valid full Kakuro (random digits satisfying no-repeat runs via the solver),
  derive the clue sums, blank the entries, and confirm the solver finds a **unique** solution
  (count solutions, abort at 2 — einstein pattern). Sizes: small/medium/large grids (e.g. 6×6,
  8×8, 10×10 including block cells).

### UI
- Draw block cells split by a diagonal with the down-sum in the upper-right and right-sum in the
  lower-left (small font); entry cells with a big centered digit.
- **Input:** tap an entry cell to select it, then tap a 1–9 keypad (bottom bar) to set it; a
  "Sudda" clears. (Reuse bullscows keypad pattern.) Optionally grey out digits already used in the
  selected cell's row-run/col-run.
- Highlight the selected cell + optionally its two runs. Show a conflict mark when a run exceeds
  its sum or repeats.
- Buttons: keypad row + "Sudda"/"Meny"; on solve "Löst!" + "Nytt pussel". Menu: 3 sizes + "Regler".
- **Splash motif:** a small Kakuro corner — one diagonal clue cell (e.g. showing sums) feeding a
  short run of digit cells.

### Gotchas
- The diagonal clue cell with two numbers is fiddly to render at cell size — prototype early in the
  emulator; keep the sum font readable.
- Uniqueness matters: a lazily-generated Kakuro often has multiple solutions. Always verify with
  the solution-counter before accepting.
- Subset-sum pruning is what keeps generation/solving fast — don't brute-force all 9^n.

---

## GAME 6 — Nurikabe ("Öar i havet")

**Elevator pitch:** paint the grid into numbered islands and one connected sea, following strict
size and shape rules.

### Rules (Swedish on rules screen)
- Grid; some cells hold a number (island **seeds**). Player paints each cell **island (white)** or
  **sea (black)**.
- Constraints: each numbered seed belongs to an island of **exactly that many** white cells; every
  island contains **exactly one** number; **islands do not touch** orthogonally (only diagonally,
  if at all); **all sea is one connected region**; and **no 2×2 block is entirely sea**.
- **Win:** all four constraints satisfied.

### game/ model + logic (ink-free, unit-tested)
- Cell state: Unknown | Island | Sea; seeds = map of position→size.
- Validators: flood-fill each white region (size == its single seed's number; exactly one seed
  per region; no two regions orthogonally adjacent); flood-fill sea (single connected component);
  scan for any all-sea 2×2. `Solved` = all cells decided AND every validator passes.
- **Solver (unique gen):** Nurikabe has known deduction rules (cells between two islands must be
  sea; a completed island is surrounded by sea; sea can't form a 2×2; "unreachable" cells that no
  island can reach become sea). Implement enough propagation to certify uniqueness, or a bounded
  solution-counter aborting at 2 (einstein pattern). This is the **hardest generator of group 4**
  — budget extra time.
- **Generator:** carve random non-touching islands of chosen sizes over a connected sea with no
  2×2 sea, place one number per island, blank the rest, verify unique. Retry loop + fallback.
  Sizes: 5×5, 7×7, 9×9 (keep small — solving cost grows fast).

### UI
- Reuse the nonogram grid layout. Seeds drawn as a number in the cell; painting: tap a
  non-seed cell to cycle Unknown → Sea(black) → Island-mark(dot) → Unknown. (Island cells can be
  left white or dotted; seeds are fixed.)
- On solve, show "Löst!". Optionally live-flag an illegal 2×2 sea block so the player sees it.
- Buttons "Rensa", "Meny"; on solve "Nytt pussel". Menu: 3 sizes + "Regler".
- **Splash motif:** a small grid with a couple of numbered white islands in a black sea.

### Gotchas
- Four interacting constraints — validate each independently and unit-test each failure mode
  (wrong island size; two islands touching; split sea; 2×2 sea block).
- **Visually close to Nonogram** (black/white grid) — lean on the seed numbers + a distinct title
  so players don't confuse the two apps.
- Generation is expensive; keep grids small and cap retries with a fallback so the app always
  starts.

---

## Definition of done (each game)
- [ ] Pure `game/` logic with unit tests (rules, solver/generator or win-detection) passing via check.ps1.
- [ ] For the puzzle games (Akari, Slitherlink, Hashiwokakero, Kakuro, Nurikabe, Towers, Singles): generator proven to emit **uniquely logic-solvable** puzzles; gen time acceptable at the largest size (cap retries + fallback so the app always starts).
- [ ] For Quarto: win-detection covers BOTH shared-1-bit and shared-0-bit lines; AI takes wins / avoids losses.
- [ ] Splash + rules screens (Swedish), menu with difficulty/mode + "Regler", per guide §10.
- [ ] All screens render cleanly in the emulator at worst-case size (verify text + layout).
- [ ] ARM `.app` built, clean filename, 8-bit BMP icon (+ `_f`), registered in view.json @Games (absolute path, string icons, name matches key).
- [ ] Deployed + confirmed on device; `*_render_test.go` removed; tree clean.
- [ ] Memory + guide updated (new project memory; bump the device app count/list).

## After all six (still missing from the 20)
Heavier strategy: **Tak** (stacking + road-building — complex rules, 3D-ish on e-ink) and
**Onitama** (5×5 with shared move-cards). **Sprouts** — skip: free-line drawing + crossing
detection is a poor fit for tap input. Optional: a standalone **Connect Four** (currently only a
variant inside "I rad").

## Group 5 — from the PocketPuzzles survey (guide §12), optional, after group 4

Not part of the "golden 20" but two more Simon Tatham puzzles worth considering — same
generate-and-verify pattern as Sudoku/Kakuro, pure tap input, no drag/animation (the eInk-hostile
class PocketPuzzles itself excludes, per guide §12). Lower priority than groups 1–4, skipped from
the priority batches only because they cognitively overlap with Sudoku/Nurikabe more than
Akari/Slitherlink/Quarto do — bump them up if you want more of that puzzle family. Same §0 setup +
splash/rules requirements as everything else. Copy `sudoku/` as the template for both.

---

### GAME 7 — Towers ("Skyskrapor" / Skyscrapers)

**Elevator pitch:** place buildings of height 1..N in a Latin square so the number of buildings
visible from each edge, looking down the row/column, matches the clue there.

#### Rules (Swedish on rules screen)
- N×N grid. Each row and column contains every height 1..N exactly once (a Latin square).
- Each of the 4 sides has clue numbers along its edges (one per row on left/right, one per column
  on top/bottom). A clue = how many buildings are **visible** looking inward from that position: a
  building is visible if it's taller than every building before it on that line of sight (a `1`
  clue means the tallest building, N, sits first; taller buildings hide shorter ones behind them).
- **Win:** grid is a valid Latin square AND every given clue's visible-count matches.
- Not all 4N edge positions need a clue — puzzles usually give a subset sufficient for a unique
  solution.

#### game/ model + logic (ink-free, unit-tested) — closest to `sudoku/`
- Board = N×N grid of 0 (empty) or height 1..N; reuse Sudoku's row/col Latin-square constraint
  checker (drop the box constraint — Towers has no sub-box rule).
- `visibleCount(line []int) int`: walk the line, count entries greater than the running max seen
  so far (skip if line has empty cells — only checkable on a complete line).
- Clues stored as 4 arrays of length N (top/bottom/left/right), 0 = no clue.
- `Solved`: Latin square valid AND `visibleCount` on every clued row/col (in the correct direction
  per side — right/bottom clues read the line reversed) equals its clue.
- **Solver:** candidate-bitmask constraint propagation + backtracking, same shape as
  `sudoku/game/solver.go` minus the box constraint, plus a visible-count pruning step: for a
  row/col with a clue, an early-filled prefix that already violates achievability (e.g. clue `1`
  but the first cell isn't `N`) can prune immediately. A bounded solution-counter aborting at 2
  certifies uniqueness (einstein pattern).
- **Generator:** fill a random valid N×N Latin square (same approach as Sudoku's full-grid
  generator), derive all 4N visible-counts, then greedily remove clues one at a time while the
  solver still proves uniqueness (same strip-and-verify loop as Sudoku/Kakuro). Sizes: 4×4 (Lätt),
  5×5 (Medel), 6×6 (Svår) — Towers puzzles get hard fast above 6, keep it modest for gen-time.

#### UI
- N×N grid in the center; clue numbers as a ring of digits just outside each edge (small font,
  same idea as Kakuro's diagonal clue cells but simpler — just one number per position, no split).
- **Input:** tap a cell to select it, then tap a 1..N keypad (bottom bar) to set the height; a
  "Sudda" clears the cell. Reuse Kakuro's/bullscows' keypad pattern (N ≤ 6 keeps the keypad small).
- Highlight the selected cell + its row/col. Grey out keypad digits already used in that row/col.
- On solve, flip any violated clue to a mark so mistakes are visible if the player fills wrong.
- Buttons: keypad row + "Sudda"/"Meny"; on solve "Löst!" + "Nytt pussel". Menu: 3 sizes + "Regler".
- **Splash motif:** 3 small building silhouettes of increasing height in a row with a `2` clue
  above them (visually explains the visibility rule at a glance).

#### Gotchas
- Direction matters: right-side and bottom-side clues read the line **reversed** (visibility looks
  inward from that edge) — a common off-by-direction bug. Unit-test all 4 sides explicitly.
- Keep N small — Towers' branching factor makes both generation and solving slower per-cell
  than Sudoku at the same N; don't reuse Sudoku's larger default sizes.
- **From PocketPuzzles' actual `games/towers.c` source (verified, corrects the estimate above):**
  their real preset ceiling is **9×9**, with an explicit source comment on WHY they stop there:
  *"the solver just isn't fast enough. Even at size 9 it can't really do the solver_hard
  factorial-time enumeration at a sensible rate."* So 9×9 is already at the edge of acceptable
  eInk generation time for their hardest tier — treat that as our ceiling too, not a floor to grow
  past; 4×4/5×5/6×6 (Lätt/Medel/Svår) as planned above is a safely conservative choice, but don't
  add a 9×9 "Svår+" tier without budgeting real generation-time testing first.
- Their difficulty tiers are hand-corrected per size (`if (diff > DIFF_HARD && w <= 3) diff =
  DIFF_HARD`) — small grids can't support their top difficulty tiers at all. If we add a size
  below 4×4 for a "very easy" mode, cap its difficulty explicitly rather than trusting the
  generator to naturally produce an easy puzzle.
- They generate by building a full Latin square first, THEN digging holes in **both** the grid
  cells and the clue values as two separate shuffled removal passes (not just stripping clues) —
  re-checking solvability after each individual removal and rolling back if it breaks solvability
  at the target tier. Same "must solve at exactly this tier, not easier" discipline as our other
  games — apply it here too (reject the puzzle if it's also solvable one tier down).
- Their tap UI uses a persistent "armed digit" numberpad pattern shared with their Sudoku
  (`solo.c`): tap a digit button to arm it, then tap grid cells to stamp that digit repeatedly
  without a popup per cell — this is their proven cross-game standard for tap-only digit entry, a
  good alternative to "select cell then tap keypad" if that flow feels slow in testing; worth
  considering for Kakuro too, since both share the same numeric-entry problem.
- Clue numbers are **never player-editable** in their implementation — only the interior cells are
  tappable; the edge clue ring is generator-only. Simplifies the input dispatch: only board cells
  need tap handling, not the clue ring.

---

### GAME 8 — Singles ("Radera dubbletter" / Hitori)

**Elevator pitch:** start from a grid full of duplicate numbers; paint cells black to remove
duplicates until every row and column has no repeats, following strict painting rules.

#### Rules (Swedish on rules screen)
- N×N grid, every cell pre-filled with a number (numbers commonly repeat within rows/cols — that's
  the puzzle). Player paints some cells **black** (removed) and leaves the rest **white** (kept).
- **Win when all of:**
  1. No number appears more than once, unpainted, in any row or column.
  2. No two painted (black) cells are orthogonally adjacent to each other.
  3. All unpainted (white) cells form a **single connected** region (via up/down/left/right steps).
- The starting numbers never change — only their painted/unpainted state.

#### game/ model + logic (ink-free, unit-tested) — closest to `nurikabe/`-style validators
- Board = N×N grid of fixed numbers (never mutated); player state = N×N bool grid `painted`.
- `noDuplicates`: for each row/col, collect unpainted cells' numbers, check no value repeats.
- `noAdjacentBlack`: scan all painted cells, fail if any orthogonal neighbor is also painted.
- `whiteConnected`: flood-fill from any unpainted cell over unpainted cells only; fail if any
  unpainted cell isn't reached (same style as Nurikabe's sea/island connectivity check).
- `Solved` = all three checks pass.
- **Solver (unique gen):** Hitori has known deduction rules — e.g. three-in-a-row same number
  forces the middle one white (else two blacks would touch or all three can't all be removed); a
  number that appears in only one place stays white; a cell flanked by two same-number cells with
  a third same number elsewhere forces specific paints. Implement enough propagation to certify
  uniqueness, or fall back to a bounded backtracking solution-counter aborting at 2 (einstein
  pattern) — Hitori's search space is small enough at these sizes that pure backtracking with the
  three validators as pruning is likely fast enough without full deduction rules.
- **Generator:** start from a random valid solved state (a white/black split satisfying all three
  rules, built by carving a connected white region + a non-adjacent black complement over a Latin
  square-ish or purely random number fill), assign numbers so duplicates appear exactly where the
  black cells are meant to resolve them, then verify the solver proves a unique solution from the
  number grid alone. This is fiddlier than Sudoku/Kakuro's "remove clues" pattern since the
  *starting* grid (the numbers) IS the puzzle, not a partially-filled one — expect more retries in
  the generation loop; cap and fall back like Nurikabe. Sizes: 5×5, 7×7, 9×9 (Hitori is dense —
  keep it smaller than Sudoku's equivalent difficulty tiers).

#### UI
- Reuse the Nonogram/Nurikabe grid layout. Every cell shows its fixed number always (numbers never
  change) with a light-grey vs. white vs. black fill for state.
- **Input:** tap a cell to cycle **white → black → white** (2-state, simpler than Nurikabe's 3-state
  cycle — Hitori has no separate "marked" state needed, though an optional "confirmed white" dot
  mark can help players track deduced-safe cells if desired).
- Live-flag violations optionally: two adjacent black cells outlined in a conflict mark; a
  duplicate unpainted number in a row/col underlined. Helps players see mistakes without solving.
- Buttons "Rensa", "Meny"; on solve "Löst!" + "Nytt pussel". Menu: 3 sizes + "Regler".
- **Splash motif:** a small 3×3 corner of a grid showing 2 black cells (non-adjacent) and a couple
  of matching numbers still visible white, hinting at the elimination mechanic.

#### Gotchas
- The three win conditions are independent and each needs its own unit test + failure-mode test
  (adjacent blacks; a duplicate slipping through; a white region split into two islands by the
  black cells) — same lesson as Nurikabe's "four interacting constraints" gotcha.
- Generation is the hard part here (see above) — budget extra time, same caveat as Nurikabe in
  group 4. Keep sizes modest and always have a fallback so the app never fails to start.
- Visually, a grid of digits with black/white cells is easy to confuse with Kakuro or Sudoku at a
  glance — lean on a distinct title and the splash motif so players don't mix up the apps.
- **From PocketPuzzles' actual `games/singles.c` source (corrects the size estimate above):**
  Singles is one of their CHEAPEST games to generate — presets go all the way to **12×12**,
  noticeably larger than their own Sudoku/Towers/Unequal ceilings, because the solver/generator is
  comparatively lightweight (no Latin-square constraint, just row/col duplicate + connectivity +
  adjacency checks). So our "keep it smaller than Sudoku" instinct above may be backwards — 9×9 as
  a "Svår" tier is likely fine or even conservative; consider testing a size above 9×9 rather than
  assuming this game needs to be small. Confirm with real gen-time measurement before committing.
- Their generator uses a **two-tier bounded retry**: an inner "reshuffle which numbers go where"
  loop capped at exactly **20** attempts, falling back to a full outer "regenerate the whole board
  from scratch" restart (unbounded) if the inner cap is exhausted. A good concrete structure to
  copy if a flat single-level retry cap proves too weak in testing.
- Their solver is named-technique-gated (`solve_singlesep`, `solve_doubles`, `solve_corners`
  always run; `solve_offsetpair`/`solve_removesplits` only at the harder tier) rather than generic
  backtracking-only — if generation struggles to find puzzles solvable by pure deduction, these
  named techniques (cells between two identical numbers must have the middle one white; a black
  cell can never be adjacent to another black cell, so isolated same-number pairs force choices)
  are the concrete deduction rules to implement, not just "some propagation."
- Their own ToDo/UX notes flag that **tap semantics need a user-facing choice**: since there's no
  natural short-tap/long-press-as-secondary-click distinction on their desktop-derived code, they
  added a `swap_buttons`-style preference letting the player choose which gesture means "paint
  black" vs. "mark known-safe." Not necessary for our simpler 2-state (black/white) cycle-on-tap
  design, but worth remembering if we add a third "confirmed white" mark state later — don't
  hardcode long-press meaning without checking it's reliable on our device first.

---
