# playtest — play a game headlessly and check the gameplay works

The screenshot emulator can render *one frame*. This harness **plays a whole
game**: it boots a game's real `app`, drives it through the real
`Init → Draw → Pointer/Key` path with injected taps and key presses, and lets a
test assert that the gameplay actually works — puzzles are winnable through the
UI, feedback is correct, an AI opponent replies, a game reaches a sane terminal
state, nothing stalls.

It runs on a normal PC (Linux/macOS/Windows with Go ≥ 1.25) — **no device, no
Docker, no cgo**. `playtest/inkemu` is a pure-Go, drop-in re-implementation of
the subset of `github.com/dennwc/inkview` the games use (framebuffer + real
TrueType text + input injection). A temporary Go workspace swaps it in for the
cgo vendor at test time; nothing tracked in git is modified.

## Run

```bash
playtest/play.sh bullscows          # play one game's tests
playtest/play.sh bullscows -v       # verbose
playtest/play.sh othello TestPlayOthelloVsAI   # a specific test
playtest/play.sh all                # emulator self-tests + every game with a play_test.go
playtest/play.sh emu                # just the emulator's own tests (playtest/inkemu)

# screenshots a test asks for land here:
PLAYTEST_SHOTS=$PWD/playtest/_shots playtest/play.sh all
```

`play.sh` builds a throwaway `go.work` that does two `use`s — the game module
and `playtest/inkemu` (whose module path *is* `github.com/dennwc/inkview`, so it
wins over the game's own `replace` to `third_party/inkview`). It passes
`-tags playtest`, so the `play_test.go` files compile **only** here — a normal
`go build`/`go vet` and the device Docker build never see them.

## Write a play test

Put it in the game's own directory as `play_test.go`, `package main`, gated by
the build tag so it never touches other tooling:

```go
//go:build playtest

package main

import (
    "testing"
    ink "github.com/dennwc/inkview"
)

func TestPlayFoo(t *testing.T) {
    a := &app{}                 // or newApp() — the game's real top-level struct
    h, err := ink.Boot(a)       // runs Init() + first Draw()
    if err != nil { t.Fatal(err) }

    h.TapXY(500, 700)           // dismiss the splash
    // ... drive the game, then assert on the real app/game state ...
}
```

Because the test is `package main` it can read the app's own unexported hit
targets (`a.buttons`, `a.keys`, `a.menuBtns`, `a.layout.CellToScreen(...)`) and
the pure `game` package — so you tap where the game actually put things and then
assert against real state, exercising the whole input→logic→display chain.

**Name every play test `TestPlay…`** — `play.sh` runs `go test -run TestPlay`, so
that prefix is how it finds them (and keeps the game's own unit tests out of the
play run).

**Don't just test the happy path.** A good play suite covers, per the game's
rules: every difficulty / board size / mode (all "sides"); the win, loss, and
tie end-states and their banners; interruptions — quitting mid-play with Back
and with the on-screen Meny button, restarting, replaying; input guards (illegal
moves rejected, no input after the game ends, taps outside the board ignored);
and each written rule checked against an **independent** computation in the test
(e.g. a from-scratch Bulls/Cows scorer, the plus-shape toggle set, the exact
disc-flips a move should produce) rather than trusting the game's own code to
agree with itself. When an end-state is hard to reach by fair play, it's fine to
construct the board directly (the tests set `a.gs.Board` to force Othello's
win/loss/tie banners and a forced-pass position) — that's still checking the real
rendering and rule logic.

### Harness API (`*ink.Harness`, emulator-only)

| Call | What it does |
|---|---|
| `ink.Boot(app) (*Harness, error)` | Reset the device, bind app, run `Init()`, render first frame |
| `h.Tap(pt)` / `h.TapXY(x,y)` | Pointer-down+up at a point; auto-redraws on `Repaint` |
| `h.TapRect(r)` | Tap the centre of a rect (a button/cell) |
| `h.Touch(pt)` / `h.TouchXY(x,y)` | Same tap via the `Touch` event path (the games' fallback handler) |
| `h.Press(key)` / `h.Back()` | Hardware key down+up (`h.Back()` = `KeyBack`) |
| `h.Texts() []TextSpan` | Every string drawn in the last frame, with its box |
| `h.FindText(s)` / `h.FindTextContains(sub)` | Locate an on-screen label (case-folded, incl. Å/Ä/Ö) |
| `h.TapText(s)` / `h.TapTextContains(sub)` | Tap a label by its text |
| `h.Screenshot(path)` | Write the current framebuffer to a PNG |
| `h.Frame()` | A copy of the framebuffer, for pixel assertions |
| `h.DrawCount()` / `h.FullUpdates()` / `h.PartialUpdates()` | Frame/flush counters for assertions |

After every injected event the harness re-runs `Draw()` for as long as the app
keeps calling `Repaint()`, so deferred work (e.g. Othello computing its AI reply
on the *next* frame, or chained AI moves when the human must pass) settles before
control returns. An app that *never* stops asking (calls `Repaint()`
unconditionally from `Draw()` — an infinite redraw loop on device) panics the
harness after 1000 chained frames, so the stall fails the test loudly.

`Boot` matches hardware: the first frame is drawn whether or not `Init` called
`Repaint` (the OS always sends a show event), and each `Boot` starts from
power-on state (white screen, default font, zeroed counters), so several tests
in one binary can't leak state into each other.

The emulator has its own test suite (`playtest/inkemu/emu_test.go`) covering the
device semantics above plus the drawing primitives, clipping (`SetClip` is
honoured, as on device), and text-span recording — run it with
`playtest/play.sh emu` (also included in `all`).

## What the shipped tests demonstrate

**All rule-based games have play suites (32 games, ~250 `TestPlay*` functions).** Every
game is won/solved through the real touch path, with its generator / AI / scoring
invariants asserted and each written rule cross-checked against an independent
computation. Run them all with `playtest/play.sh all`. Two games' solvers were
extended to expose the answer they already compute (`akari.SolveBulbs`,
`hashiwokakero.SolveBridges`) — additive, behaviour-preserving, consistent with
the sibling games whose solvers already return their result.

Highlights that go well past the happy path:

- **bullscows** (9) — wins on all three difficulties; scoring driven through the
  keypad and checked against an independent from-the-rules scorer; the
  distinct-digit rule (used keys grey out) and "no Gissa until complete"; Sudda
  erase; unlimited guesses (no loss state); quit with Back and with Meny then
  restart; replay after a win; the rules screen.
- **lightsout** (8) — solves all three board sizes through the grid (via the
  game's GF(2) solver); the toggle rule verified cell-by-cell (interior press
  flips the 5-cell plus, corner flips 3, edge flips 4); "Losning" overlay equals
  the solver's cells; "Ny" reshuffles a fresh solvable puzzle; taps outside the
  grid and after a win are ignored; quit; the rules screen.
- **othello** (8) — legal vs illegal moves and exact disc-flips checked against a
  pure `Apply`; the win / loss / tie banners (via constructed terminal boards); a
  crafted forced-pass position (White genuinely has no move, verified with
  `HasMove`, and "Pass!" renders); two full deterministic games vs the AI; a full
  hotseat game driving **both** colours; quit; the rules screen. Writing the
  first Othello play-through surfaced a real stall bug (no AI move was queued when
  the human was forced to pass); the fix is in `othello/main.go`.

## Files

- `inkemu/` — the pure-Go `ink` package (framebuffer, TTF text, input injection,
  screenshot). Only used by the harness; device builds are unaffected.
- `play.sh` — the runner (workspace swap + `go test -tags playtest`).
- `_shots/` — screenshots written by tests (git-ignored).
