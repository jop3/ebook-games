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
playtest/play.sh all                # every game that has a play_test.go

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

### Harness API (`*ink.Harness`, emulator-only)

| Call | What it does |
|---|---|
| `ink.Boot(app) (*Harness, error)` | Bind app, run `Init()`, render first frame |
| `h.Tap(pt)` / `h.TapXY(x,y)` | Pointer-down+up at a point; auto-redraws on `Repaint` |
| `h.TapRect(r)` | Tap the centre of a rect (a button/cell) |
| `h.Press(key)` / `h.Back()` | Hardware key down+up (`h.Back()` = `KeyBack`) |
| `h.Texts() []TextSpan` | Every string drawn in the last frame, with its box |
| `h.FindText(s)` / `h.FindTextContains(sub)` | Locate an on-screen label |
| `h.TapText(s)` / `h.TapTextContains(sub)` | Tap a label by its text |
| `h.Screenshot(path)` | Write the current framebuffer to a PNG |
| `h.DrawCount()` / `h.FullUpdates()` | Frame/flush counters for assertions |

After every injected event the harness re-runs `Draw()` for as long as the app
keeps calling `Repaint()`, so deferred work (e.g. Othello computing its AI reply
on the *next* frame, or chained AI moves when the human must pass) settles before
control returns.

## What the shipped tests demonstrate

- **bullscows** — splash → menu → type guesses on the keypad → win; checks the
  Bulls/Cows feedback shown matches the rules, and that "Gissa" is withheld
  until the entry is complete.
- **lightsout** — starts a scrambled puzzle and *solves it* by tapping the grid
  (using the game's own GF(2) solver), proving the puzzle is winnable via the UI
  and win-detection fires.
- **othello** — plays a full game against the built-in AI to a terminal state,
  checking turn alternation, the deferred AI reply, and that the winner matches
  the disc counts. Writing this test surfaced a real stall bug (no AI move was
  queued when the human was forced to pass); the fix is in `othello/main.go`.

## Files

- `inkemu/` — the pure-Go `ink` package (framebuffer, TTF text, input injection,
  screenshot). Only used by the harness; device builds are unaffected.
- `play.sh` — the runner (workspace swap + `go test -tags playtest`).
- `_shots/` — screenshots written by tests (git-ignored).
