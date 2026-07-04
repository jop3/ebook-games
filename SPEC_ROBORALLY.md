# Build spec — Robo Rally (simplified, solo vs. AI) for PocketBook Verse Pro

> **Status: implemented** in `roborally/`. Pure logic + generator + blind AI in
> `roborally/game/` (unit-tested via `go test ./game/`), UI in `main.go`/`ui.go`/
> `screens.go`, play suite in `play_test.go` (`playtest/play.sh roborally`), ARM
> `.app` builds via the SDK Docker image. This spec is the design of record; a few
> v1 details were simplified further during the build (noted inline where relevant).


Hand-off spec for a **simplified Robo Rally** port: **1 human vs. 1–3 AI robots**, race to touch
checkpoints in order across a hazard-filled factory floor. Programming is secret and simultaneous;
resolution is register-by-register and fully deterministic. This design leans *into* what e-ink is
good at — the game is naturally turn-based and step-wise, so no animation is needed; each register
step is a discrete frame.

Target device and toolchain as in **`POCKETBOOK_GAMEDEV_GUIDE.md`** — read it in full first. This
spec assumes 1072×1448 portrait (effective drawable height **1340**), greyscale, **tap-only**,
32-bit ARM, `dennwc/inkview` SDK, the inkstub/inkrender emulators, and the `playtest/` harness.

---

## 0. READ FIRST (non-negotiable setup — identical to every game in the library)

- **Read `POCKETBOOK_GAMEDEV_GUIDE.md` in full.** Toolchain, ink API, every trap (§0–§12).
- New Go module `roborally/` — **copy `othello/` as the template and gut it** (it already has the
  AI-vs-human loop, `game/` split, splash/rules, play-test shape).
- **All rules + physics + AI + generator live in `roborally/game/`** with NO `ink` import, so they
  unit-test cgo-free via `scratchpad/check.ps1 roborally`. UI/drawing in `ui.go`; `main.go`
  implements `ink.App`.
- **Input:** taps in `Pointer` on `PointerUp`, `Touch` on `TouchUp` as fallback, both → one
  `handleTap(p image.Point)`; dispatch by screen.
- **Fonts once** in `Init()` into a `*Fonts` struct; never `OpenFont` inside `Draw`.
- **ScreenSize** only inside `Draw()`/`Init()`, never the constructor. Seed structs with 1072×1448.
- **`ink.Repaint()` at the end of `Init()`** (else the first tap is dead).
- **Redraw cadence:** full-screen redraw each `Draw`; `FullUpdate()` on every screen/state change
  and on each resolution step (clears ghosting — important, the board changes a lot), else
  `PartialUpdate`.
- **Splash + full Swedish rules screens are mandatory** (guide §10). `screenSplash` is the initial
  state. Motif = a robot glyph on a tile with a conveyor chevron + a checkpoint flag, all line-art.
- **Layout against effective height 1340**, bottom-anchored UI built bottom-up (guide §5).

---

## 1. Scope — what the simplified version keeps and drops

**Keep (the core loop that makes RR *Robo Rally*):**
- Secret simultaneous programming: draw a hand, commit **5 cards** into 5 ordered **registers**.
  **Every robot — human and AI alike — commits blind.** No one sees anyone else's cards until they
  reveal, register by register. Collisions, pushes, and crashes are surprises to *everyone*,
  including the AI that caused them. The AIs are **not allowed to peek** at each other's or the
  human's committed program when planning (§6 makes this a hard, enforced invariant, not a promise).
- Register-by-register activation in **priority order** (distance to the priority antenna).
- Board elements: **conveyor belts (single + express), rotating gears, walls, wall lasers, pits,
  repair sites, checkpoints**.
- **Collisions push** robots (chained); shove enemies into pits / off the edge.
- **Damage** → smaller hand next round; death → respawn at last checkpoint with +2 damage.
- Race: first to touch the **last checkpoint in order** wins.

**Drop / defer for v1 (add back in v2, §10):**
- The 30-second real-time timer — solo play is turn-based, no timer. (The "stress" it created is
  reproduced in the **AI human-error model**, §6, which is where the fun of RR's chaos comes from.)
- Option/upgrade cards (power-ups), flags-as-shields, virtual robots, team play.
- Register **locking** at 5+ damage — spec'd as an optional toggle, off by default in v1.
- Pushers / crushers / gears-that-are-also-belts and other exotic tiles.

**Player counts:** 1 human + **1–3 AI** (2–4 robots total). No hot-seat human multiplayer in v1
(the whole point is a good solo experience; multiplayer adds no logic but a lot of UI).

---

## 2. Data model (`roborally/game/`, ink-free)

```go
type Dir uint8 // N, E, S, W
func (d Dir) Left() Dir; func (d Dir) Right() Dir; func (d Dir) Opposite() Dir
func (d Dir) Step() (dx, dy int) // N = (0,-1)

type Card uint8
const ( Move1 Card = iota; Move2; Move3; BackUp; RotR; RotL; UTurn )
// Each drawn card also carries a Priority int (tie-break within equal antenna distance).

type Gear uint8 // GearNone, GearCW, GearCCW

type Tile struct {
    Kind       uint8 // Floor, Pit, Repair
    Belt       Dir   // belt direction; BeltNone sentinel when no belt
    BeltExpress bool  // express belts move an extra step (see §4)
    Gear       Gear
    Walls      uint8 // bitmask N|E|S|W
    Laser      Dir   // wall laser fires in this dir from this tile; LaserNone when absent
    LaserCount uint8 // 0,1,2 barrels (damage per hit)
    Checkpoint uint8 // 0 = none, else 1..N (must be touched in order)
    StartDock  uint8 // 0 = none, else dock index
    Antenna    bool  // priority antenna (exactly one per board)
}

type Board struct {
    W, H   int
    Tiles  []Tile // row-major, len W*H
    NCheck uint8  // number of checkpoints
    Antenna image.Point
}

type Robot struct {
    Pos        image.Point
    Facing     Dir
    Damage     int   // 0..9
    NextCheck  uint8 // checkpoint we still need (1..NCheck), then win when > NCheck
    ArchivePos image.Point // respawn point (last checkpoint / start dock)
    ArchiveDir Dir
    Alive      bool
    IsHuman    bool
    Profile    AIProfile // §6; ignored for the human
    ID         int
}

type Phase uint8 // PhaseProgram, PhaseResolve, PhaseDone

type GameState struct {
    Board     *Board
    Robots    []Robot
    Round     int
    Phase     Phase
    Registers [][5]Card // per robot
    Hands     [][]Card  // per robot, this round's draw
    CurReg    int       // 0..4 during resolution
    Log       []string  // human-readable step log ("Reg 3: du → fram 2, band → öst, laser −1")
    Winner    int       // -1 until someone wins
    LockRegs  bool      // v2 toggle
    rng       *rand.Rand
}
```

Keep the hand deck realistic: a fixed **program deck** per robot (RR's distribution, scaled down —
e.g. Move1×18, Move2×12, Move3×6, BackUp×6, RotR×18, RotL×18, UTurn×6), drawn and reshuffled from
the discard when short. Hand size = `9 - Damage` (min 1). This deck-limits both human and AI to the
same authentic constraint — you often *can't* play the move you want, which is itself a source of
good chaos and keeps the AI honestly imperfect (§6).

---

## 3. Board encoding & GRAPHICS (greyscale, tap-only, line-art)

**Greyscale rule (guide §3):** render distinctions as **patterns and line-art, not shades** — e-ink
muddies/ghosts grey fills. **Do NOT use font arrow glyphs** for belts/gears — `◂ ▸ ◄ ►` render as
broken boxes in the device font (guide §5a). Draw every arrow/chevron with `DrawLine`.

### Cell size & fit
Square cell `= min(availW/cols, availH/rows)`, centered. Board sizes: **8×8 (~120px cells, fast),
10×10 (~100px, default), 12×12 (~85px, large)**. Reserve a top status strip (~80px) and a bottom
control strip (~120px); board fills the middle. Verify the **worst case (12×12)** in the emulator —
element glyphs must stay legible at 85px.

### Tile art (all line-art / thick strokes)
| Element      | Drawing                                                                       |
|--------------|-------------------------------------------------------------------------------|
| Floor        | thin cell border only                                                         |
| Wall         | **thick black bar** on the tile edge (inflate 3–4px) — the clearest glyph     |
| Conveyor     | a **chevron** pointing in belt dir, drawn with two `DrawLine`s                 |
| Express belt | **double chevron** (two stacked) — visibly "faster"                           |
| Gear         | a **circular arc + arrowhead** (CW/CCW), line-art                              |
| Pit          | filled black square with a white X, or dense hatch — unambiguously "danger"    |
| Repair       | a **wrench / "+"** drawn from lines                                            |
| Laser emitter| a small nub on the wall + (during resolution only) a thin beam line across it   |
| Checkpoint   | bordered tile with a **big bold number 1/2/3** + a flag notch                  |
| Antenna      | a small **tower** mark (vertical line + radiating ticks) on its tile           |
| Start dock   | a faint number in the corner                                                   |

Keep static clutter down: draw laser **beams only during the laser sub-step** of resolution, not on
the idle board (emitters always show).

### Tile layering & the busy-tile cap (so a crowded tile stays legible)
Multiple elements share a tile without turning to mush because each owns a **different region** of
the cell, drawn back-to-front in a fixed **z-order**:
1. **Interior background:** belt chevron / express double-chevron OR gear arc OR pit hatch OR repair
   wrench OR checkpoint number (these are mutually exclusive on a tile — a tile is one *floor kind*).
2. **Edges:** walls (thick bars) and laser emitters (small nubs) — always on the cell border, never
   colliding with the interior glyph.
3. **Top:** the robot, drawn at ~70% of the cell with its heading nose.
So the worst legible tile is e.g. "express belt + a wall on one edge + a robot on top" — three
non-overlapping regions, fine even at 85px. **Generator legibility cap: at most ONE interior
element + at most TWO wall/laser edge features per tile.** A checkpoint or pit tile carries **no belt
or gear** (you shouldn't be conveyed off your own goal, and it keeps the goal tile clean). Verify the
worst case in the emulator (§9): a 12×12 board with a robot standing on an express belt next to a
laser emitter and a wall must still read clearly.

### Robot glyphs — must show BOTH identity and FACING
Reuse irad's identity approach (distinct marks, not shades) but add a **heading nose**:
- Robot body = a shape with a clear pointed "nose" drawn in `Facing` direction (e.g. a house/chevron
  pentagon, or a square with a triangular nose on the facing edge).
- Identity by **fill pattern**: P1 solid black, P2 hollow ring, P3 hatched, P4 checker — plus a small
  ID digit (1–4) in the body. Patterns read cleanly on e-ink where 4 grey shades would not.
- A dead robot (mid-round, awaiting respawn) draws as a faint dashed outline on its archive tile.

### The two screens
1. **Program screen** (`screenProgram`): status strip (round, your damage, "Mål: ✓1 ✓2 →3"), then
   **5 register slots** in a row, then your **hand** (up to 9 card buttons in a grid). Tap a hand
   card → it fills the next empty register; tap a filled register → clears it back to hand. A
   **"Kör"** button (enabled only when all 5 filled) starts resolution. Card faces are line-art:
   Move-N = up-arrow + digit, BackUp = down-arrow, RotR/RotL = curved arrow, UTurn = 180° arrow;
   each shows its priority number small in a corner.
2. **Resolve screen** (`screenResolve`): full board + a thin strip showing **"Register k/5"** and the
   **Log** line for the last sub-step, plus a **"Nästa"** button. Default pacing is **tap-to-advance
   per register** (predictable on e-ink, ~5 taps/round). Each register advances through its
   sub-steps (§4) accumulating into one settled frame with a text log of what happened; an optional
   **"Steg för steg"** menu toggle splits it into move → board → laser frames for players who want to
   watch the collisions. `FullUpdate()` on every step.

Result banner on `screenDone` ("Du vann!" / "Robot 2 vann"), with round count and each robot's
checkpoint progress.

---

## 4. Resolution order (deterministic — this IS the game; get it exactly right)

Per round, after all 5 registers are locked in, for **each register k = 1..5**:

1. **Priority order.** Sort robots by **Manhattan distance to the priority antenna** (closest =
   highest priority = moves first); break ties by the register card's `Priority` value, then by ID.
2. **Reveal & move.** In priority order, each living robot executes its register-k card:
   - `RotR/RotL/UTurn`: change `Facing` only.
   - `Move1/2/3`: step forward that many tiles, **one tile at a time**, checking walls on both the
     leaving and entering edges. Moving into an occupied tile **pushes** the chain (recursively) if
     no wall blocks it; robots pushed off the board edge or onto a **pit** die. `BackUp`: one tile
     opposite to facing (no rotation), also pushes.
   - A robot that leaves the board or lands on a pit **dies immediately** (queued for respawn at
     round end) — a dead robot skips its remaining registers this round.
3. **Board elements**, in this fixed order (RR-authentic, simplified):
   a. **Express belts move 1**, then **all belts (express + single) move 1** — so express carries 2,
      single carries 1 per register. A belt entering a wall or another (stationary) robot's tile is
      blocked; two robots converging on one tile — neither moves (RR rule). Belt curves rotate the
      robot to the new belt's direction on entry.
   b. **Gears** rotate the robot on them 90° (CW/CCW).
   c. **Wall lasers fire**: each emitter beams in its direction until a wall/robot; first robot hit
      takes `LaserCount` damage.
   d. **Robot lasers fire**: each living robot beams forward one line; first robot hit takes 1.
   e. **Repair sites / checkpoint touches** heal/register (see 4-below).
4. **Checkpoint & repair touch (end of register).** If a robot sits on **its `NextCheck`** checkpoint,
   `NextCheck++` and its archive is updated to here (also heal 1). Landing on a repair site heals 1
   and updates archive. Touching checkpoints **out of order does nothing**. Reaching `> NCheck`
   sets `Winner` and ends the game immediately.

**End of round:** respawn the dead — reset `Pos/Facing` to `Archive*`, `+2` damage, `Alive = true`.
Discard used registers, draw new hands (`9 - Damage`), back to `PhaseProgram`.

Implement each of these as a small pure function (`applyMove`, `pushChain`, `stepBelts`,
`spinGears`, `fireWallLasers`, `fireRobotLasers`, `touchCheckpoints`) so the play-tests can assert
each against an **independent reference computation** (guide §6b: test the rulebook, not the happy
path).

---

## 5. Level generation

Ship **3 hand-authored fixed courses** (guaranteed-good first experience) **and** a **"Slumpbana"**
procedural generator for replayability. The generator follows the library's universal rule:
**generate → verify → regenerate on failure** (guide §12).

### Generator algorithm (`GenerateCourse(size, difficulty, nCheck, seed) *Board`)
1. Empty grid; the board edge is lethal (falling off = death).
2. **Belts as rivers, not noise.** Carve a few directed "belt paths" (random walks with momentum) so
   conveyors form coherent lanes with a consistent direction; mark a subset as express. This is what
   makes belts feel designed rather than random — critical for fairness and readability.
3. **Pits** scattered at density `d_pit(difficulty)`, but never on a belt tile that would create an
   inescapable dump, and never blocking *all* routes between consecutive checkpoints (checked in 7).
4. **Gears** at some belt bends / open junctions.
5. **Wall lasers** on a few edges aimed down open lanes (more, and double-barrel, at higher difficulty).
6. **Walls** to form chokepoints near checkpoints, so the final approach demands alignment (a turn +
   a move), not a straight sprint — this is the main *difficulty* lever, not board size.
7. **Checkpoints** placed spaced apart (min Manhattan gap ∝ size); **priority antenna** central;
   **start docks** on the bottom edge per the **Start-line contract** below.
8. **Verify (regenerate on any failure):**
   - **Reachability:** BFS from each checkpoint to the next over non-lethal tiles must succeed.
   - **Solvable by a competent programmer:** run the §6 planner (Expert profile, fumble off) from a
     start dock; it must reach every checkpoint within a round budget `R(size)`. If it can't → discard.
   - **Not trivial (difficulty grading, PocketPuzzles-style):** the *direct* straight path to each
     checkpoint must be blocked by a wall/pit/hazard so the solution needs ≥K turns/detours. If a
     Move-forward-only program would reach it, the course is too easy for its tier → regenerate.
   - **No belt-death loops:** simulate an idle robot dropped on each belt tile; it must not be
     conveyed into a pit/off-edge without any card able to save it.
   - **Fairness of starts:** guaranteed by the Start-line contract below (bottom-row, spaced,
     centered, clear launch zone, random assignment) — regenerate if no valid dock window exists.
9. Seed the RNG explicitly (`rand.New(rand.NewSource(seed))`) so a course can be **replayed by seed**
   and every play-test is reproducible. Difficulty knobs (`d_pit`, express ratio, laser count, wall
   chokepoint count, round budget) are a small table keyed by an easy/medium/hard course setting —
   **separate from AI difficulty** (§6), so a beginner can play a hard course with clumsy AIs, or vice
   versa.

### Start-line contract (MANDATORY for every course — fixed, generated, or future)
The four start docks are not scattered — they form a fair, uniform starting line. **Any course
generator or hand-authored board must satisfy all of these**, or robots stall/crash on turn one:

1. **Bottom edge, one row.** All docks sit on the bottom row (`y = H-1`), so the whole field is
   "ahead" of every racer and nobody starts facing off the board.
2. **All face up (North).** Uniform heading into the board; no dock faces a wall or the edge.
3. **Spaced with a free gap between neighbours.** Docks occupy *every other* column
   (`x, x+2, x+4, x+6` → a 7-wide block for 4 docks), never adjacent — so two robots can't collide
   on the first move and each has side room.
4. **Centered.** The dock block is placed as close to the horizontal centre as a valid window allows
   (ties broken randomly), not jammed against the left edge.
5. **Clear launch zone.** The dock row **and the row directly ahead** are cleared of all hazards
   (pits, belts, gears, lasers) and walls across the block's span, so no robot has a block or wall in
   front of it — a `Move 1` always succeeds from the start.
6. **Random robot→dock assignment.** Which robot (including the human) starts in which dock is a
   fresh `rng.Perm` each game, so positions vary while the line stays fair. The *block position* is
   centered/deterministic; only the *assignment* is random.
7. **Reachable & no checkpoint/antenna clobbered.** The chosen window must not overlap a checkpoint
   or the antenna, and dock1 → cp1 must pass the reachability check (step 8).

This is enforced by `placeBottomDocks` (generator), the `NewGame` dock shuffle, and the
`TestStartDocksHaveRoom` play/unit test (asserts, across every difficulty and many seeds: a
non-lethal, unwalled tile in front of each dock, and no two docks adjacent). **Keep this invariant
when adding new tile types, larger boards, or hand-authored courses.**

Fixed courses live as literal tile arrays in `game/courses.go` (compact rune-map + a decoder, e.g.
`.` floor, `>` belt-east, `»` express-east, `O` pit, `+` repair, `1` checkpoint-1, `#`+edge for
walls, `L^` laser-north, `A` antenna, `a` dock). The same decoder can load emulator test fixtures.

### 5a. Difficulty progression & element budget (how conveyors/traps/lasers scale)
Difficulty is driven by **which elements appear, how densely, and how adversarially they're placed** —
*not* mainly by board size. Elements unlock as the tier rises, so a beginner learns one mechanic at a
time and a hard course weaponizes all of them. The generator reads a per-tier **element budget**:

| Tier (Bana-svårighet) | Board | Checkpts | Elements introduced (cumulative)                          | Density / teeth |
|-----------------------|-------|----------|-----------------------------------------------------------|-----------------|
| **Lätt** ("Träning")  | 8×8   | 2–3      | walls, **single conveyors**, checkpoints, a few **pits**   | wide lanes; pits kept *off* the natural path; no express, no gears, no lasers |
| **Mellan**            | 10×10 | 3        | + **express belts**, **gears**, **single-barrel lasers**, chokepoints | pits placed *beside* belt lanes (belts become a threat); one laser lane crosses an approach |
| **Svår** ("Dödsfabrik")| 12×12| 4–5      | + **double-barrel lasers**, **belt→pit** routes, **gear-at-chokepoint**, laser **crossfire** | pits flank checkpoint approaches; express belts overshoot goals; hazards actively fight you |

**Difficulty is placement, not just count.** The generator makes hazards *matter* by pointing them at
where you want to go — this is what turns "elements on a board" into "traps":
- **Conveyors as hazards, not scenery:** at Mellan+, route a belt so a careless robot is carried
  *toward* a pit or *past* its checkpoint (overshoot), so you must plan a counter-move or step off the
  belt. Express belts (2/register) near a checkpoint create real overshoot pressure.
- **Pits are the trap system:** beyond raw density, place pits where a belt would dump a mis-timed
  robot (**belt→pit**), and flanking the last tile before a checkpoint so a sloppy approach dies. The
  **no-belt-death-loop** verify (§5 step 8) still guarantees a *card exists* to save you — it's a trap,
  not a guaranteed kill.
- **Lasers gate the approach:** aim emitters **down the natural lane to a checkpoint**, so you must
  time your program to not be standing in the beam on the wrong register — or take the damage on
  purpose to keep pace. Double-barrel at Svår makes that toll bite (2 dmg = a card lost next round).
- **Gears at chokepoints** spin you off-heading right where you need precise alignment, forcing an
  extra corrective register.
- **The "not trivial" grade (§5 step 8) enforces engagement:** if a straight Move-forward program
  would reach a checkpoint, the course is regenerated — so the tier's hazards are on the *only* viable
  path, not decoration you can skirt.

Because course difficulty and **AI difficulty (§6) are separate knobs**, every combination is valid:
a gentle Träning board against Expert bots, or a Dödsfabrik against clumsy Nybörjare bots that keep
crashing into its pits (often the funniest mode). The element budget is a small literal table in
`game/difficulty.go` so it's trivial to retune after playtesting.

---

## 6. AI difficulty & the human-error model (the fun)

The chaos and humor of tabletop Robo Rally come from **stressed humans mis-programming under the
timer** — robots U-turn into pits, register the turn one slot too late, sprint off the edge. A solo
port must *manufacture* that, or the AIs are either coldly optimal (unbeatable, no comedy) or
uniformly random (stupid, no tension). The design is two layers: a **competent planner** plus a
**stress-driven fumble model** layered on top.

### Layer 0 — Information firewall (the AIs are blind too — enforce it, don't just intend it)
This is the load-bearing fairness rule: **an AI plans using only public information** — the board,
every robot's **current** position/facing/damage/next-checkpoint, and its **own** hand — and **never**
the *committed programs* of any other robot (nor its own future-round hands). Nobody knows anyone
else's cards in advance, so nobody's plan may depend on them. Make this **structurally impossible to
violate**, not a matter of discipline:
- `PlanProgram` takes a **read-only public snapshot** (board + robots' current visible state + this
  robot's hand), **not** the full `GameState`. The `Registers` and `Hands` of other robots are simply
  **not in the argument** the planner can see. If it isn't passed in, it can't be cheated with.
- When the planner **simulates a candidate program forward** to score it (§4 physics), it runs that
  simulation **against the other robots held stationary at their current tiles** — i.e. it treats
  them as fixed obstacles, exactly as a blind human must guess. It does **not** simulate their moves,
  because it doesn't know them. Its expectations about collisions/pushes will therefore sometimes be
  wrong — which is the point: the AI gets nudged, blocked, and shoved into pits by moves it never saw
  coming, same as everyone else. That surprise is a *feature*, and it stacks with the §2 hand limit
  and the fumble model (below) to keep AIs believably fallible.
- A regression play-test (§8) **asserts independence**: the program `PlanProgram` returns is
  **bit-identical** whether or not the other robots' `Registers`/`Hands` are populated in the game
  state — proof the planner draws on zero hidden information.

### Layer 1 — Planner (competence)
Simulation-greedy, one round deep (board elements make deeper lookahead noisy and it's the honest
human horizon anyway):
1. From the robot's **actual drawn hand** (already a real constraint — you often can't do the ideal
   move), enumerate candidate 5-card programs. The hand caps the branching; prune with a beam
   (keep top-M partial programs by heuristic) so it stays snappy on the device CPU.
2. For each candidate, **run the deterministic resolver forward with opponents held stationary**
   (Layer 0 — belts/gears/lasers/pushes against the *current* board, not opponents' hidden moves) and
   **score the resulting position**:
   - `− distance(pos → NextCheck)` after the round (dominant term; use BFS distance over the board,
     not Manhattan, so it respects walls),
   - big penalty for ending **dead** (pit/edge) or facing a wall/pit it will next step into,
   - penalty for **laser damage** taken; bonus for **damage dealt / shoving an enemy toward a pit**,
   - small bonus for **touching a checkpoint** this round and for ending **aligned** to the next leg.
3. Pick the best-scoring program. Profiles reweight these terms (below).

Because the hand is random and the horizon is one round, even the "best" plan is imperfect and
varied — the planner alone already plays like a decent-but-fallible human. That's intentional.

### Layer 2 — Human-error / "stress" model (applied AFTER planning)
With probability `p_fumble`, perturb the chosen program the way a rushed human would:
- **Register slot-swap** (most common): swap two adjacent registers. "I put the turn in slot 3, not
  slot 2." Small effect usually, occasionally disastrous.
- **Wrong card:** substitute a plausible-but-worse card from the hand into one register.
- **Panic re-order:** shuffle the last two registers.
- **Over-commit:** keep an obviously bad card the planner would have avoided in one slot.

`p_fumble` = `base(difficulty) + stress`, where **stress rises with the exact pressures that rattle a
human**: adjacent to a pit or the board edge, took damage last round, currently behind in the race,
or **within one leg of a checkpoint** (choking near the finish). This makes AIs **crack precisely
when the stakes are highest** — dramatic, funny, and it hands the player openings at the moments that
feel earned. It literally re-creates "stressed humans make mistakes."

### Difficulty tiers (menu: AI-svårighet — independent of course difficulty)
| Tier              | Planner                                   | `base` fumble | Stress sensitivity | Feel                              |
|-------------------|-------------------------------------------|---------------|--------------------|-----------------------------------|
| **Nybörjare**     | shallow beam, ignores lasers/pushing      | ~0.35         | high               | clumsy, crashes a lot, very human |
| **Van** (default) | full scoring, pit-avoidant                | ~0.15         | medium             | the "fun default" — beatable      |
| **Expert**        | full scoring incl. shoving + laser-dodge  | ~0.04 (>0!)   | low                | tough, but still fumbles rarely   |

Keep Expert's base fumble **nonzero** — a robot that never errs isn't Robo Rally and isn't fun to
beat. Difficulty is set **per game** (all AIs share the tier) with an optional per-robot
**personality** flavor (cheap scoring presets, makes 3-AI games lively):
- **Rusher** — over-weights forward progress; higher stress near hazards (sprints, sometimes off a cliff).
- **Bully** — over-weights shoving enemies toward pits; will detour to ram you.
- **Cautious** — over-weights survival; slower, hoards safe moves, rarely dies.

### Determinism & testability
Planner + fumble model draw from `GameState.rng` (seeded), and the whole thing is pure `game/` code,
so play-tests (§8) can assert reproducible AI behavior: an Expert AI reliably clears a solvable
course within budget; a Nybörjare AI demonstrably dies/fumbles more; **no profile ever emits an
illegal or incomplete 5-card program**; and fumbles fire at the expected stress thresholds.

---

## 7. Screen flow & UI states

```go
const (
    screenSplash screen = iota // initial (guide §10)
    screenMenu
    screenProgram              // pick 5 cards into registers
    screenResolve              // step registers on the board
    screenDone                 // result banner
    screenRules
)
```
- **Menu:** Course (Bana 1/2/3 or **Slumpbana**), Board size (8/10/12), Course difficulty
  (Lätt/Mellan/Svår), **# AI (1–3)**, **AI-svårighet** (Nybörjare/Van/Expert), **Starta**, **Regler**.
- **Program → Kör → Resolve → (loop back to Program each round) → Done.**
- Hardware **Back**: from Rules/Program/Resolve/Done → Menu (confirm-abandon from mid-game is fine to
  skip; just drop to menu). Meny button on the status strip does the same.
- Splash motif: robot glyph on a tile + a conveyor double-chevron + a checkpoint "1" flag — line-art.
- Rules screen: **full Swedish rules** — explain registers, priority (antenna), the board-element
  order (band → kugghjul → lasrar), pushing, pits/respawn (+2 skada), damage → färre kort, and the
  win condition (checkpoints i ordning).

---

## 8. Play-test plan (`roborally/play_test.go`, `//go:build playtest`, `package main`)

Every function `TestPlay…`; drive the real `Init→Draw→Pointer` path via the `playtest/inkemu`
harness (`ink.Boot`, `h.TapRect`, `h.TapText`, `h.FindTextContains`, `h.Back`). Cover the **whole
rulebook**, asserting against **independent reference computations**, not the game agreeing with
itself (guide §6b):

- **Program via UI:** tap 5 hand cards into registers, assert `Registers` matches the taps; **"Kör"
  disabled** with <5 filled; tapping a filled register returns the card to the hand.
- **Movement/pushing:** construct a state, run one register, assert final positions match a
  from-scratch mover — incl. a **push chain** (A shoves B shoves C), a wall **blocking** a push, and
  a shove **into a pit** (victim dies).
- **Board elements:** independent checks for express-carries-2 vs single-carries-1, a **belt curve
  rotating** the robot, converging belts **both blocked**, a **gear** rotation, a **wall laser**
  dealing `LaserCount`, a **robot laser** dealing 1.
- **Death/respawn:** step off the edge and onto a pit → dies, respawns at **archive with +2 damage**
  at round end; hand size next round = `9 - Damage`.
- **Checkpoints:** touching out of order does nothing; in order advances `NextCheck` and updates
  archive; reaching the last one sets `Winner` and ends the game (banner shows).
- **Priority:** two robots contest a tile; assert the antenna-closest moves first (and the card
  `Priority` tie-break).
- **Generator:** for each difficulty, `GenerateCourse` output passes its own verifier (reachable,
  Expert-solvable within budget, not straight-line trivial, no belt-death loop) over many seeds.
- **AI:** Expert clears a fixed solvable course within budget over N seeds; Nybörjare dies/fumbles
  strictly more often; **no illegal/incomplete program** from any profile/hand; fumble fires at the
  expected stress threshold under a fixed seed.
- **AI blindness (anti-cheat, load-bearing):** assert `PlanProgram`'s output is **bit-identical**
  with the other robots' `Registers`/`Hands` populated vs. cleared — the planner must draw on **zero
  hidden information** about opponents' committed cards. Also assert the planner scores candidates
  with **opponents held stationary** (feed it a state where an opponent *would* move into it next
  register and confirm the plan is unchanged — the AI can't have foreseen it).
- **Flow/guards:** quit via **Back** and **Meny** from Program and Resolve; restart/replay; no input
  accepted after `Winner` is set; taps off the board/hand ignored.

Expose any answer a solver hides (e.g. `game.SolveDistance`, the planner's chosen program) so tests
can assert it — additive and behaviour-preserving, as done for `akari.SolveBulbs` /
`hashiwokakero.SolveBridges` (guide §6b).

---

## 9. Verify + ship (guide §6, §7, §8)

- **Unit tests** for the resolver, generator, planner, fumble model via `check.ps1 roborally` (cgo-free).
- **Render** every screen to PNG with `render.ps1 roborally <Test> OUT=<dir>` — menu, splash, rules,
  program (full hand + registers), resolve (mid-collision, laser beams drawn), done — at the **worst
  case (12×12, 4 robots)**. The emulator shows real TTF text; verify checkpoint numbers, card faces,
  and that belt/gear **line-art** (not font glyphs) reads clearly. Confirm bottom controls have margin.
- **Build** ARM `.app` via the Docker SDK image; confirm `ELF 32-bit LSB … ARM`.
- **Icons + view.json:** 8-bit BMP `roborally.bmp`/`roborally_f.bmp` via `scratchpad/mkicon/`; add
  `U_roborally` (absolute path, string-form icons, name-matched `roborally.app`) to **@Games** via
  `scratchpad/vjfinal/main.go`. Device re-reads view.json on USB-disconnect.
- **Delete** any `*_render_test.go` before shipping; keep `play_test.go` (regression guard).
- **On-device glyph check (guide §5a):** confirm the belt/gear line-art and the robot heading-nose
  read correctly on the actual e-ink panel — draw them as lines, never as `◂ ▸` font glyphs.

---

## 10. v2 / stretch ideas (out of scope for v1)

- Register **locking** at 5+ damage (`LockRegs` toggle already stubbed).
- **Option/upgrade cards** (power-ups) — the deck and a "buy on repair" step.
- Real-time **timer mode** for the human (recreates on-table panic; the AI already simulates it).
- **Share-a-seed** UI so two devices can race the same generated course asynchronously.
- More tile types (pushers, crushers, dual-speed gears), larger multi-checkpoint courses, and an
  MCTS planner for a genuinely brutal Expert tier.
- Hot-seat multi-human (pure UI work; logic already supports N robots).

---

### Why this is a good fit (one paragraph, for the reader)
Robo Rally resolves in discrete register steps with no continuous motion, so e-ink's "static frame
per step" is a feature, not a compromise; the board is a tap-legible grid of line-art tiles; the
physics and AI are pure deterministic Go that slot straight into the existing `game/`-package +
`play_test.go` pattern; and the signature *fun* — robots doing dumb things under pressure — is
reproduced faithfully by the stress-driven AI fumble model instead of a real-time timer. It would be
the most mechanically distinctive game in the library, built entirely on tools already in the repo.
