# Build spec — Text Adventure engine + Colossal Cave port ("Grottan")

Hand-off spec for adding a **tap-driven text-adventure** to the library — a new *category*
alongside the logic puzzles. The plan: build one small generic engine, ship it first with a
freely-licensed classic (**Colossal Cave Adventure**), and keep the door open to drop in more
stories later (dunnet, an original, Scott-Adams-format games).

**Why this fits the device:** an e-reader is built to render text; the whole game output *is*
word-wrapped prose (we already have `wrapText`/`splitWords` from guide §10). No animation, no
refresh-rate fight, no generator/solver math. The hard part of the puzzle games (generation +
difficulty grading) simply doesn't exist here — the cost moves to **content ingestion + a good
tap UI**.

**The one real design problem and its fix:** a classic adventure uses a *typed parser*
(`get lamp`, `go north`). Typing on e-ink is miserable and it's unclear the Go `inkview` wrapper
even exposes the firmware soft-keyboard (every existing game is tap-only Pointer events). **So we
do not build a parser.** We replace it with a **tap verb + tap noun** interface (the Scott Adams
two-word model, which Colossal Cave already fits). This sidesteps the keyboard entirely and is
friendlier than a 1976 parser.

---

## 0. READ FIRST (non-negotiable setup — same as every game)

- **Read `POCKETBOOK_GAMEDEV_GUIDE.md` in full first.** Toolchain, the verified `ink` API, and
  every trap. This spec assumes it.
- Target: PocketBook Verse Pro (PB634), **1072×1448 portrait, greyscale, tap-only**, 32-bit ARM.
  Lay out against **effective height 1340** (guide §5), reserve a ~46px top margin (§5a).
- **Input:** taps in `Pointer` on `PointerUp`, `Touch` on `TouchUp` as fallback, both →
  one `handleTap(p image.Point)`. Dispatch by screen. (guide §4)
- **Fonts:** open every (typeface,size) ONCE in `Init()` into a `*Fonts` struct; reuse via
  `SetActive`. NEVER `OpenFont` inside `Draw`. (guide §4)
- **ScreenSize** inside `Draw()`/`Init()`, never in the constructor. End `Init()` with
  `ink.Repaint()`. Full-screen redraw each `Draw`; `FullUpdate()` on state change / every ~6–8
  frames to clear ghosting, else `PartialUpdate`.
- **Module layout:** copy an existing game as a template and gut it. Keep ALL engine logic in a
  subpackage with **no `ink` import**, so it unit-tests cgo-free via `scratchpad/check.ps1`.
  Folder name: **`grottan/`** (Swedish for "the cave"); binary `grottan.app`.
- **MANDATORY splash + rules screens** (guide §10). `screen` enum with `screenSplash` first, then
  `screenMenu`, `screenGame`, `screenRules`. Motif = simple line-art (see §7). Full Swedish rules.

---

## 1. Content — what to use, and the licensing (READ CAREFULLY)

The whole reason this is viable is that the genre's canonical work is **free to adapt**. But
"free to play" ≠ "free to ship" for some of these — the split below matters.

### ✅ PRIMARY: Colossal Cave Adventure via **Open Adventure** (ESR) — ship this first
- Repo: **`gitlab.com/esr/open-adventure`** (GitHub mirror exists too). This is Eric S. Raymond's
  modernization of the 1976 Crowther & Woods original, **released under a BSD-2-Clause license with
  Crowther's and Woods' explicit approval.** Permissive, commercial-friendly, **attribution-only.**
- **The gold nugget: `adventure.yaml`.** The entire game world — rooms, objects, vocabulary, the
  travel table, all message text — is a single structured YAML data file. We are NOT
  reverse-engineering a binary; we ingest a documented data model (see §3). ESR's own build
  generates C from this YAML via `make_dungeon.py` — we mirror that idea and generate **Go**.
- Obligation: **keep the BSD-2 license notice** in the repo (a `grottan/THIRDPARTY.md` or header)
  and show a one-line credit on the in-app rules/about screen. Trivial.

### ✅ ALTERNATIVE data source (public domain): **troglobit/adventure**
- A C port under the **Unlicense** (public domain). Game data is embedded in `src/*.c`, not a clean
  YAML — **less convenient to ingest** than Open Adventure. Use only as a cross-reference for
  behaviour, or if you want a strictly PD provenance. Prefer Open Adventure's YAML.

### ✅ LATER story option (GPL): **dunnet**
- The Zork-style adventure shipped inside GNU Emacs (`lisp/play/dunnet.el`, Ron Schnell),
  **GPLv2+**. Freely reusable *if we accept copyleft* on that game module. Small, self-contained.
  Good as a *second* story once the engine exists. (Data is Emacs Lisp → hand-transcribe into our
  data model; it's small.)

### ⚠️ ENGINE TEMPLATE, not content: **Scott Adams format (ScottKit / ScottFree)**
- The **format and tooling are open** (ScottFree by Alan Cox, ScottKit by Mike Taylor — both GPL;
  the `.dat` format is fully documented). BUT **the classic game *files* (Adventureland, Pirate
  Adventure, …) remain © Scott Adams** — "freely downloadable" is NOT a redistribution license.
  So: borrow the two-word `VERB NOUN` engine *design* and, if we want, author an **original** story
  in that shape — do **not** bundle the original Scott Adams `.dat` files.

### ⛔ AVOID: Infocom Zork I–III
- Owned by Activision, fully copyrighted. The Z-machine interpreter (Frotz) and format are open, but
  **the story files are not free.** Same engine-vs-content split, wrong side of it. Do not ship.

### ⛔ AVOID: bundling any "free to play but © retained" IF
- Much of the IF Archive (`ifarchive.org`) is free-to-play but not free-to-relicense. If you ever
  pull a story from there, confirm an explicit open/CC license per-work first.

**Net: build the engine, ingest `adventure.yaml` from Open Adventure (BSD-2), keep the notice,
credit on the about screen. Everything else is a future story.**

---

## 2. Scope & phasing (Colossal Cave is BIG — do NOT try to one-shot it)

The full Colossal Cave is ~140 rooms, ~60 objects, a lamp-battery limit, dwarves/pirate with combat
& RNG, magic words (XYZZY/PLUGH/PLOVER), a maze, hints, and a pile of hand-coded "special" actions.
A faithful full port is weeks of work and most of it is the special-case logic, not the map.

**Phase the build so the app is shippable early and each phase is emulator-verifiable:**

- **Phase 1 — Engine + surface world (MVP).** Data model (§3), tap UI (§4), save/restore (§5),
  splash+rules (§7). Ingest only the **above-ground + first cave rooms** (well house, grate, debris
  room, first few chambers), the lamp, keys, grate, cage/bird/rod, and the plain `goto` travel
  rules + `LIT`/lamp darkness. No dwarves, no RNG, no maze, magic words optional. This proves the
  engine end-to-end and is a real, playable game. Ship it.
- **Phase 2 — Full map + objects + treasures + scoring.** Ingest all rooms/objects, the treasure
  set, the lamp-battery turn limit, death/resurrection, final score & rank classes. Still skip the
  agent AI if it's fighting you.
- **Phase 3 — Agents & flavour.** Dwarves/pirate/combat (RNG), hints, the maze, all `special`
  actions. This is the long tail; only do it if Phase 2 feels worth completing.

Track coverage explicitly — when the ingester skips rooms/actions, **`log()` what was dropped** so
"MVP" never masquerades as "complete."

---

## 3. Engine data model (ink-free package `grottan/story`)

Ingest `adventure.yaml` **at build time** into Go literals (no on-device YAML parser — keep the
binary dependency-free and the logic cgo-free-testable, mirroring how the puzzles keep logic pure).
Write a tiny generator (see §6) that emits `story/storydata_gen.go`.

### 3.1 The Open Adventure data shapes we ingest

From `adventure.yaml` (verified structure):

- **`locations`** — each has `description: {long, short, maptag}`, `conditions:` a set of flags
  (`LIT`, `FLUID`, `NOBACK`, `DEEP`, `FOREST`, `ABOVE`, …), and a **`travel:`** list.
- **`objects`** — each has `words:` (vocabulary synonyms), `inventory:` (carried label),
  optional `states:` + `descriptions:` (per-state text) + `changes:`, `locations:` (start),
  `immovable:`, `treasure:`.
- **`travel` entries** are rules of the shape
  `{verbs: [MOTION…], cond: [kind, args…], action: [kind, target]}`:
  - `verbs` — motion words that trigger this rule (`NORTH`, `DOWN`, `ENTER`, `XYZZY`, `PLUGH`, …).
  - `cond` — one of `[pct, N]` (N% chance), `[carry, OBJ]`, `[with, OBJ]`, `[not, OBJ, STATE]`,
    `[nodwarves]`, or absent (unconditional).
  - `action` — `[goto, LOC_X]` (move), or `[special, N]` (hand-coded), or a message id.
- **`arbitrary_messages`, `actions`, `hints`, `classes`, `turn_thresholds`, `obituaries`,
  `motions`** — the rest of the text and metadata.

### 3.2 Go representation (design these; keep them small and value-typed)

```go
package story

type LocID int
type ObjID int
type Motion int   // enumerated movement/magic words (N,S,E,W,NE,…,UP,DOWN,IN,OUT,XYZZY,PLUGH,…)

type CondKind int
const (
    CondNone  CondKind = iota
    CondPct            // Pct% random
    CondCarry          // player carries Obj
    CondWith           // Obj is here (carried or in room)
    CondNotState       // Obj is NOT in State
    CondNoDwarves      // Phase 3; treat as always-true earlier
)

type Cond struct {
    Kind  CondKind
    Obj   ObjID
    State int
    Pct   int
}

type ActKind int
const (
    ActGoto ActKind = iota   // move to Dest
    ActSpecial               // hand-coded routine #N (Phase 3)
    ActMessage               // just print Msg, don't move
)

type Travel struct {
    Verbs  []Motion
    Cond   Cond
    Act    ActKind
    Dest   LocID
    Msg    int   // index into Messages, for ActMessage / blocked moves
    Sp     int   // special routine number
}

type Location struct {
    Long, Short string
    Lit         bool      // LIT flag (else dark without a lit lamp)
    Forest      bool      // (and other flags as needed)
    Travel      []Travel
}

type Object struct {
    Words        []string  // vocabulary → matched to noun buttons
    Inventory    string    // label when carried / listed
    States       []string  // e.g. ["LAMP_DARK","LAMP_BRIGHT"]; empty = stateless
    Descriptions []string  // per-state room description ("" = invisible in that state)
    Start        LocID     // initial location (LOC_NOWHERE if none)
    Start2       LocID     // some objects start in two places (e.g. huge parts); optional
    Immovable    bool
    Treasure     bool
}

// Emitted by the generator:
var Locations []Location
var Objects   []Object
var Messages  []string   // arbitrary_messages, action defaults, etc.
```

### 3.3 Mutable game state (the save payload — see §5)

```go
type State struct {
    Loc, OldLoc LocID
    Carried     map[ObjID]bool
    ObjAt       map[ObjID]LocID   // current location of each movable object
    ObjState    map[ObjID]int     // current state index per object
    Known       map[string]bool   // discovered magic words / story flags
    Turns       int
    LampLife    int               // Phase 2: battery countdown
    Score       int
    Dead        bool
    Won         bool
}
```

### 3.4 Engine functions (pure; the whole point is these unit-test with `check.ps1`)

- `New() *State` — place objects at their `Start`, lamp dark, player at LOC_START.
- `Describe(s) []string` — location long/short (short after first visit — track a `visited` set on
  `State`) + a line per visible object at the room + "It is now pitch dark…" if `IsDark`.
- `IsDark(s) bool` — `!Locations[loc].Lit && !(carrying lamp && lamp state == BRIGHT)`.
- `Exits(s) []ExitButton` — evaluate the current room's `Travel` rules whose `Cond` passes and
  `Act==ActGoto`; dedup to one button per compass/vertical/in-out direction (label from the Motion).
  This is what the UI renders as tappable exits. **Magic-word motions (XYZZY…) are NOT auto-shown**
  — they're puzzle knowledge; surface them only via the "Säg…" verb once `Known`.
- `VisibleObjects(s) []ObjID` — objects with `ObjAt==loc` (and a non-empty description for their
  state) plus carried objects. These become noun buttons.
- `Move(s, m Motion) []string` — find the first passing `Travel` rule for `m`; apply
  goto/special/message; increment `Turns`; decrement `LampLife`; return narration.
- `Act(s, v Verb, n ObjID) []string` — the verb table: `TAKE, DROP, OPEN, CLOSE, LOCK, UNLOCK,
  LIGHT, EXTINGUISH, LOOK/EXAMINE, WAVE, EAT, DRINK, THROW, READ, INVENTORY`. Each checks
  reachability/immovable/state and mutates `State`. Unknown verb+noun → a stock "Jag förstår inte…"
  message. Keep the verb set small in Phase 1 (`TAKE, DROP, OPEN, UNLOCK, LIGHT, EXAMINE, LOOK,
  INVENTORY`) and grow it.
- `Score(s) int` / `Rank(s) string` — Phase 2, from `classes`.

**Determinism for tests:** RNG (`pct` conds, Phase 3 agents) must go through an injectable source,
NOT `Math.rand` global — and remember the guide's note that `Math.random()`/`Date.now()` are
unavailable in some tooling contexts; seed from `State.Turns` or an explicit field so replays and
unit tests are deterministic.

---

## 4. The tap UI — parser replaced by verb + noun (this is the whole trick)

No typing. Layout in `ui.go` (imports `ink`), against **1072×1340**, bottom-anchored bottom-up
(guide §5). Four zones top→bottom:

```
┌───────────────────────────────────────────┐  top margin ~46
│  Platsnamn (short room name)   ·  [Meny]   │  header strip
├───────────────────────────────────────────┤
│                                           │
│   TRANSCRIPT (word-wrapped prose)         │  large scrollable text area
│   – current room description              │  body font ~36, lineH ~48
│   – result of the last action appended    │  swipe / ▲▼ to scroll (guide §5a)
│                                           │
├───────────────────────────────────────────┤
│  UTGÅNGAR:  [Norr][Öster][Ner][In] …      │  exits row (from Exits())
│  HÄR:       [lampa][bur][spö] …           │  nouns present (VisibleObjects)
│  [Titta][Ta][Släpp][Öppna][Tänd][Säg…]    │  verb bar (armed-verb model)
│  [Ryggsäck]                               │  inventory button
└───────────────────────────────────────────┘  bottom margin ~60
```

**Interaction model (Scott-Adams "armed verb", same idea as Sudoku's armed-digit stamping):**
1. Tap an **exit** button → `Move`; transcript updates; screen redraws. (Most common action = one
   tap.)
2. Tap a **verb** → it highlights (armed). Then tap a **noun** button (a "HÄR" object or an
   inventory item) → `Act(verb,noun)` runs, verb disarms. Tapping the armed verb again cancels.
3. Verbs needing **no noun** (`Titta`/LOOK, `Ryggsäck`/INVENTORY) execute immediately on tap.
4. **`Säg…`** (SAY) opens a small list of **discovered** magic words → tap one → treated as a
   Motion. Words appear here only after `Known[word]` is set (found in room text / a puzzle), so
   discovery stays a puzzle, not a menu giveaway.
5. **Scrolling:** track pointer-down Y; on up, if `|Δy| ≥ ~110px` scroll the transcript instead of
   dispatching a tap (guide §5a). Keep ▲/▼ buttons too.

**Rendering notes:**
- Fit button labels to the button; fall back to a smaller font, ellipsize last (guide §5a). Keep
  verb bar to ≤6 buttons; short Swedish labels.
- Avoid glyphs that break in the device button font (`◂ ▸ ◄ ►`); use words. `▲ ▼` are fine (§5a).
- Redraw whole screen each `Draw`; `FullUpdate` on room change (clears ghosting), `PartialUpdate`
  while only the transcript scrolls.
- Keep object/exit button rects on the struct each frame so `handleTap` can hit-test them (same
  pattern as mastermind's `button.hit(p)` / einstein's `btnHit`).

**Language:** narration text comes from the (English) Open Adventure data. Decide up front:
either **(a) ship English narration** with a **Swedish UI chrome** (buttons/menus/rules) — lowest
effort, and English is fine for a cave classic; or **(b) translate the room/message text to
Swedish** during ingestion — much more work, do only if desired. Recommend **(a) for Phase 1**.
Rules screen is Swedish regardless (guide §10).

---

## 5. Save / restore (adventures are long — this is mandatory, unlike the puzzles)

- Serialize `State` (§3.3) to a single file under the app's data dir. Use Go `encoding/gob` or a
  simple hand-rolled text format — **keep it in the ink-free package** so it's testable without the
  device. One slot is enough for v1 ("Fortsätt" on the menu resumes); add named slots later.
- Autosave on every successful `Move`/`Act` and on `Close()`. On launch, if a save exists, the menu
  shows **"Fortsätt"** above "Nytt spel".
- Guard against a corrupt/old save (version byte) → fall back to a fresh game, never crash.

---

## 6. Build-time ingester (`scratchpad/advgen/`)

A small standalone Go (or Python, matching ESR's `make_dungeon.py`) tool — **runs on the dev
machine, not the device**:

1. Read `adventure.yaml` from a local checkout of Open Adventure.
2. Map its `locations`/`objects`/`travel`/`arbitrary_messages` into the §3.2 Go structs.
3. Emit `grottan/story/storydata_gen.go` as Go literals (`var Locations = []Location{…}` etc.),
   plus an ID-constant block (`const LOC_START LocID = …`, `const OBJ_LAMP ObjID = …`) generated
   from the YAML anchor names so the hand-written engine can refer to objects/rooms by name.
4. For Phase 1, accept an **allow-list** of location/object IDs and emit only those (+ rewrite
   travel destinations that leave the subset into a "you can't go that way yet" message). `log`
   every dropped room/action.
5. Copy Open Adventure's `LICENSE` into `grottan/THIRDPARTY.md` and add the attribution line used
   on the about screen.

Keep the generator out of the shipped module (it's a scratchpad tool); only the generated
`storydata_gen.go` is committed. Regenerate when widening coverage between phases.

---

## 7. Splash + rules (guide §10 — mandatory, same as every game)

- `screenSplash` is the initial state; tap → menu.
- **Motif** (`DrawSplash` line-art, monochrome): a simple **cave mouth / grate** — an arch of a few
  strokes with vertical grate bars, or a lantern glyph. Keep it icon-simple like the other games'
  motifs. Also make the 8-bit BMP launcher icon from it (`scratchpad/mkicon/`).
- **Menu:** "Fortsätt" (if a save exists), "Nytt spel", "Regler". Title "Grottan".
- **`DrawRules`** (full Swedish): explain the tap model — tap an **utgång** to move; tap a **verb**
  then a **föremål** to act; **Säg…** for magic words you've discovered; **Ryggsäck** for
  inventory; the game **auto-saves**. Note the goal (explore the cave, collect treasures, return).
  Add the required **credit line**: *"Baserat på Colossal Cave Adventure av Will Crowther & Don
  Woods, via Open Adventure (Eric S. Raymond), BSD-2-Clause."*

---

## 8. Verify + ship (guide §6, §7, §8)

- **Unit-test the engine** with `scratchpad/check.ps1 grottan`: a fresh `New()`, a scripted walk
  (well house → get lamp/keys → unlock grate → descend → `IsDark` true without lamp, false after
  `LIGHT`), TAKE/DROP moving objects between room and inventory, and a save→load round-trip that
  reproduces identical `State`. These are pure and cheap — write them first.
- **Render every screen** to PNG with the inkrender emulator (`scratchpad/render.ps1 grottan …`):
  splash, menu, rules, a room with several exits+objects, a long transcript scrolled, dark-room
  message. **Emulator now shows real TTF text (guide §0a/§6)** — verify wrapping, that the verb bar
  and exit/noun rows fit with bottom margin at the **worst case** (a room with many exits + many
  objects + a long description). Check no bottom overflow (guide §5).
- **Build** ARM `.app` via the Docker SDK image (guide §7); confirm `ELF 32-bit ARM`. Ship under a
  clean filename `grottan.app` matching the `U_grottan` view.json key.
- **view.json** entry under @Games (guide §8 recipe: absolute path, string-form icons,
  name-matched, no `param`). 8-bit BMP `grottan.bmp` + `grottan_f.bmp`.
- Delete any `*_render_test.go` before shipping.

---

## 9. Effort estimate & recommendation

- **Phase 1 (engine + tap UI + surface world + save + splash/rules): the bulk of the value.**
  Architecturally *easier* than the recent puzzle games on the hard axis — **no generator, no
  difficulty grading, no solver.** The new work is the ingester (§6) and the transcript/verb UI
  (§4). A focused build.
- **Phase 2** is mostly data volume + a few systems (lamp battery, scoring, death). Mechanical.
- **Phase 3** (dwarves/pirate/RNG/maze/specials) is the long tail — optional, do only if wanted.

**Recommendation: build Phase 1 end-to-end and ship it.** It delivers a real, legally-clean,
marquee text adventure and — because the engine is data-driven — the *same* engine later accepts
dunnet, an original story, or a fuller Colossal Cave with only new data, not new code. One port,
reusable engine, clean BSD attribution.

### Follow-ups / future stories (engine reuse, no new engine code)
- Fuller Colossal Cave (Phases 2–3).
- **dunnet** (GPLv2 module) — hand-transcribe its small world into the data model.
- An **original** short adventure authored in the Scott-Adams two-word shape (no IP risk).
- Swedish translation pass of the narration text (Phase 1 ships English narration + Swedish UI).
