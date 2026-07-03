# PocketBook Verse Pro — Game Dev Guide

Everything learned building **I rad**, **Mastermind**, **Black Box**, and **Einsteins Gåta**
for the PocketBook Verse Pro (PB634). Read this before building a new game — it lets you
skip every trap we hit the hard way.

Target device: **PocketBook Verse Pro (PB634)**, e-ink, **1072×1448 portrait**, greyscale only,
32-bit ARM. SDK: [dennwc/inkview](https://github.com/dennwc/inkview) (Go wrapper over libinkview).

---

## 0. TL;DR — the rules that will bite you

0. **⚠️ Real drawable height is ~1340, NOT the 1448 `ScreenSize()` reports.** Content below ~1360
   wraps to the top. Lay out against effective height **1340**. This is the #1 layout trap. (§5)
1. **Taps come as `Pointer` events, not `Touch`.** Handle `Pointer` on `PointerUp`. (§4)
2. **Open fonts ONCE in `Init()`, never inside `Draw`.** Per-frame `OpenFont` = sluggish + dropped taps. (§4)
3. **The device caches apps by filename.** To reload after a change, give it a NEW name (`foo_v2.app`). (§7)
4. **Icons are via `view.json` + 8-bit BMPs**, not `<app>.app.bmp`. (§8)
5. **The emulator renders REAL text** and now **auto-flags off-screen / margin overflow** against the
   real 1340 height — run the bounds audit on every screen (§6, `scratchpad/BOUNDS_AUDIT_HOWTO.md`).

---

## 0a. Emulator text — now REAL (TrueType), was garbled before

**Fixed 2026-07:** the `inkrender` emulator now renders text with a real system TrueType face
(Windows `arial.ttf`/`arialbd.ttf`), so preview PNGs show **correct, legible text** — full words,
proper bold, Swedish å/ä/ö, and even ■/□/→ symbols. Titles/labels/wrapping can now be verified
from the screenshot, not just layout. `ink.StringWidth` in the emulator also uses the real face,
so measured centering/wrapping matches what you see.

**History (why old screenshots look broken):** before the fix, the emulator used a tiny built-in
ASCII bitmap font with only ~30 glyphs and printed `?` for everything else — "Othello" →
"O?H?LLO", å/ä/ö mangled. If you see an OLD screenshot full of `?`, that's the pre-fix bitmap
font, not an app bug. Re-render with the current emulator to see the true text.

**Fallback:** if no system TTF can be loaded, the emulator silently falls back to the old bitmap
font (garbled but positioned). So `?`-filled text in a *fresh* render means "TTF didn't load"
(check the font paths in `inkrender/font.go`), not an app bug. On the real device text is always
correct regardless — it uses the device's own fonts. See §6 for how the TTF path is wired.

---

## 1. Environment (already set up on this machine)

- **WSL2 + Ubuntu 26.04** with **Docker Engine** inside it. Sudo password: `noviso`
  (not passwordless — prefix docker: `echo noviso | sudo -S -p "" docker ...`).
- **Portable Go 1.23.4** at
  `C:\Users\tenka\AppData\Local\Temp\claude\...\scratchpad\goroot\go` (session scratchpad).
  If a new session's scratchpad differs, re-download `go1.xx.windows-amd64.zip` and unzip.
- Device mounts as **`D:`** (volume `PB634`); apps live in `D:\applications\`.
  It disconnects frequently — always check `ls /d/applications` before copying.

PowerShell env for the portable Go:
```
$root="<scratchpad>"
$env:GOROOT="$root\goroot\go"; $env:GOPATH="$root\gopath"
$env:Path="$env:GOROOT\bin;$env:Path"; $env:CGO_ENABLED="0"; $env:GOFLAGS="-mod=mod"
```

---

## 2. Project layout

Each game is its own Go module under `C:\github\Ny mapp\<game>\`:
```
<game>/
  go.mod                       module <game>; replace inkview => ./third_party/inkview
  main.go                      implements ink.App (menu + game loop + events)
  <logic>.go                   PURE game logic — must NOT import ink (so it unit-tests cgo-free)
  ui.go / render.go / ...      drawing; imports ink
  third_party/inkview/         vendored SDK (copy of the cloned repo + a go.mod)
```
`go.mod`:
```
module <game>
go 1.21
require github.com/dennwc/inkview v0.0.0
replace github.com/dennwc/inkview => ./third_party/inkview
```
Get `third_party/inkview` by cloning `https://github.com/dennwc/inkview` and copying its
`*.go *.h *.c LICENSE` in, plus a `go.mod` that says `module github.com/dennwc/inkview`.
(The upstream repo has no go.mod.) Keep logic in a subpackage or ink-free files so you can
test it without the SDK.

---

## 3. The verified `ink` API (package name `ink`)

Entry: `func main() { ink.Run(app) }` where app implements:
```go
Init() error; Close() error; Draw()
Key(ink.KeyEvent) bool; Pointer(ink.PointerEvent) bool
Touch(ink.TouchEvent) bool; Orientation(ink.Orientation) bool
```
Events:
- `ink.PointerEvent{ image.Point; State }` — states `PointerUp/Down/Move/Long/Hold`. **Taps = PointerUp.**
- `ink.TouchEvent{ image.Point; State }` — `TouchUp/Down/Move` (rarely fires on this device).
- `ink.KeyEvent{ Key; State }` — `KeyStateUp`; keys incl. `KeyBack` (hardware Back).

Graphics (all `image`/`image/color`): `ClearScreen()`, `DrawRect(r, cl)`, `FillArea(r, cl)`,
`DrawLine(p1, p2, cl)`, `InvertArea(r)`. Colors: `Black, White, DarkGray, LightGray`.
**Greyscale only** — render "colors" as distinct patterns/symbols/digits.

Text: `f := ink.OpenFont(ink.DefaultFont, size, true)` (also `DefaultFontBold`);
`f.SetActive(ink.Black)`; `ink.DrawString(pt, s)`; `ink.StringWidth(s)`; `f.Close()`.

Screen: `ScreenSize()` → `image.Pt(1072,1448)`; `Repaint()` (queues Draw);
`FullUpdate()` (clean, slow); `PartialUpdate(r)` (fast). Do full-screen redraw each Draw;
call `FullUpdate` on state changes / every N frames to clear e-ink ghosting, else `PartialUpdate`.

---

## 4. Input & performance — the two bugs that cost us the most

**Pointer, not Touch.** libinkview delivers finger taps as `EVT_POINTER*` → `App.Pointer()`.
An app that only implements `Touch()` gets ZERO response (renders fine, nothing tappable;
only hardware Home works, because the OS handles it). Correct pattern:
```go
func (a *app) Pointer(e ink.PointerEvent) bool {
    if e.State != ink.PointerUp { return false }
    return a.handleTap(e.Point)
}
func (a *app) Touch(e ink.TouchEvent) bool {   // fallback
    if e.State != ink.TouchUp { return false }
    return a.handleTap(e.Point)
}
func (a *app) handleTap(p image.Point) bool { /* dispatch by screen */ }
```

**Fonts once, not per frame.** Opening a font is expensive on e-ink. Calling `OpenFont`+`Close`
inside draw helpers (per peg/row/button/label) → 30–40+ opens/frame → slow redraw → the app
feels stuck on launch and DROPS taps that land during a redraw. Fix: open every (typeface,size)
you need ONCE in `Init()`, store on the struct, reuse with `SetActive` before `DrawString`.
Close them in `Close()`. (Black Box's `InitFonts()`/`*Fonts` struct is the template.)

---

## 5. Layout — the drawable area is 1072×**1340**, NOT 1072×1448

**⚠️ CRITICAL, ruler-tested on device (2026-07): `ink.ScreenSize()` returns 1072×1448 but the
real drawable height is only ~1360. Anything drawn below ~1360 WRAPS AROUND to the top of the
screen** (y=1300 shows at the bottom; y=1400 appears at the TOP). This caused the whole
"bottom buttons appear at the top / half off both edges" saga (Nim, Jotto, Einstein). Anchoring
bottom UI to `H=1448` makes it WORSE — it pushes content straight into the wrap zone.

**Rule: lay out against an effective height of 1340, never `ScreenSize().Y`.**
```go
const usableH = 1340                 // real drawable height; ScreenSize().Y (1448) lies
W := ink.ScreenSize().X              // width ~1072 is fine
H := usableH                         // use THIS for all vertical layout
```
Width (~1072) is trustworthy; only height is wrong.

Then, within 1072×1340, derive ALL positions from W/H, never hardcode pixels that can overflow:
- **Build bottom-anchored UI BOTTOM-UP, never top-down with hardcoded y.** This bug recurred
  (Einstein, then Nim): stacking `boardBot:=H-320; ctrlY:=boardBot+30; btnY:=ctrlY+bh+30; hint at
  btnY+bh+30` accumulates until the last row spills past `H`. Instead: `margin:=H/24` (~60px);
  `hintY:=H-margin`; `btnY:=hintY-gap-btnH`; `ctrlY:=btnY-gap-bh`; then `boardBot:=ctrlY-gap`.
  Every bottom element gets guaranteed margin to the edge. Reserve a top margin ~60px too (§5a).
- Board/grid: cell = `min(availW/cols, availH/rows)`; center it; never draw under the button bar.
- Button bar at the bottom with a margin; always fully on-screen.
- Long text (clue lists etc.): reserve its block height first, then fit the grid in what's left;
  stop with a `…` guard above the button bar.
- Left-side row labels: bind each to its grid row's y so they align and never overlap.
Verify with the emulator (§6) at the WORST case (largest board / most items). The emulator now
renders real text, so bottom overflow IS visible in the PNG — render the game screen and check
the bottom edge has margin.

### 5a. UI ergonomics learned on-device (Läsordning feedback pass, 2026-07)
Real on-device testing surfaced these; bake them in from the start:
- **Reserve a top margin (~46px).** Titles/buttons drawn at `y≈0` sit under the status strip
  and are hard to tap. Push the top title bar and its divider down by a `topMargin` constant;
  don't let anything interactive touch `y=0`.
- **Fit button labels to the button — never assume they fit.** A fixed-size label like
  "Hämta hela serien" overflowed its cell into the neighbour. Helper: try the normal button
  font, fall back to a smaller one if `StringWidth(label) > cellW-16`, ellipsize only as a last
  resort. Keep 3–4 buttons max per bar; prefer short labels ("Hämta serie" not "Hämta hela
  serien").
- **Glyphs like `◂ ▸ ◄ ►` render as a broken/missing box** in the device's button font (same
  class of issue as å/ä/ö in the emulator, but this one is on-DEVICE). `▲ ▼` DO render. Use a
  plain word ("Tillbaka") instead of a left-triangle. Test any non-ASCII glyph on hardware before
  relying on it.
- **Add swipe scrolling, not just buttons.** The Verse Pro screen is touch. Track the
  pointer-down Y; on pointer-up, if `|Δy| ≥ ~110px` treat it as a scroll (rows ≈ `-Δy/rowH`)
  instead of a tap; otherwise dispatch the tap. Wire the SAME logic into both `Pointer` (Down/Up)
  and `Touch` (Down/Up). Keep the ▲/▼ buttons too — some users prefer them.

---

## 6. Headless emulator (debug without the device)

Two pure-Go stubs of `ink` in scratchpad:
- `scratchpad/inkstub/` — no-op; for fast `go build`/`go vet`/logic `go test`. Driven by
  `scratchpad/check.ps1 <game>` (uses the portable Go). Add new symbols here when a game
  references an ink function the stub lacks (`undefined: ink.X` → add `X`). We've added
  `ink.Pad`, `ink.Exit`, etc. over time — mirror the real API, don't change the game.
- `scratchpad/inkrender/` — draws into an `image.RGBA`; `ink.Canvas()` returns it. A
  `*_render_test.go` calls the game's draw funcs (or `ink.Run(app)` + tap injection via
  `app.Pointer(ink.PointerEvent{Point:p, State:ink.PointerUp})`) then saves `ink.Canvas()` as
  PNG. Driven by `scratchpad/render.ps1 <game> <TestName> <ENVVAR=outdir>`.

**Emulator text = real TrueType (since 2026-07).** `inkrender/font.go` loads a Windows system
font (`arial.ttf` / `arialbd.ttf`, bold chosen when the OpenFont name contains "bold"/"-b") via
`golang.org/x/image/font/opentype`, caches a face per (size,weight), and `DrawString`/
`StringWidth` use it. `DrawString` positions by TOP-LEFT (baseline = y + ascent) to match the
real SDK contract, so emulator layout == device layout. Falls back to the old bitmap font only
if no TTF loads.

**Toolchain gotcha:** `x/image` needs a COMPLETE stdlib (`image/draw`). The session's portable
Go 1.23.4 has an EMPTY `src/image/draw/` (partial extraction) → `package image/draw is not in
std`. `render.ps1` therefore points `GOROOT` at the cached full toolchain
`gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.windows-amd64` (has full stdlib + supports
x/image v0.43). `inkrender/go.mod` requires `golang.org/x/image v0.43.0` (needs go ≥1.25). If a
new session lacks that toolchain, `go get golang.org/x/image/font/opentype@latest` from any
module auto-fetches it (that's what seeded it here); or re-extract a clean portable Go so
`image/draw` isn't empty and pin `x/image` to a version matching that Go.

Swap a stub in for a run, then restore (both .ps1 scripts do this automatically):
```
Copy-Item go.mod go.mod.bak
[System.IO.File]::WriteAllText("$PWD\go.mod", (Get-Content go.mod -Raw) -replace './third_party/inkview','<stub-path>')
go test . -run TestRender   # or go build ./... etc.
Move-Item go.mod.bak go.mod    # ALWAYS restore before the Docker build
```
Delete `*_render_test.go` before shipping (they reference `ink.Canvas()`/`ink.SetScreenSize()`
which the real SDK lacks; `go build` for the .app ignores `_test.go`, but keep the tree clean).

---

## 7. Build the .app and install it

The `.app` is a cgo ARM binary; libinkview + the vendor clang live only in the Docker image.
Write a bash script to scratchpad and run it (PowerShell mangles inline heredocs):
```bash
echo noviso | sudo -S -p "" docker run --rm \
  -v "/mnt/c/github/Ny mapp/<game>:/app" -w /app \
  sunsung/pocketbook-go-sdk:latest build -o <game>.app .
file <game>.app     # must say: ELF 32-bit LSB executable, ARM, EABI5
```
Run from PowerShell: `wsl -e bash -lc "sed 's/\r$//' '<wslpath>' | bash"`.

**Install:** copy `<game>.app` to `D:\applications\`, `sync`, verify sha256 matches.
**Device caches by filename** — after ANY change, ship under a NEW name (`<game>_v2.app`, `_v3`, …)
so the launcher loads it fresh; delete the old one. Then eject + reboot the device.

---

## 8. Icons & names (view.json) — SOLVED. Follow the recipe EXACTLY.

Registering apps in `/system/config/desktop/view.json` gives them a custom name + icon in the
app list. It works — but only if you get ALL of the following right. Any single mistake and the
app appears in the list but **tapping it bounces back to the home screen** (this exact trap cost
us multiple sessions; the silent culprit was relative paths). Proven by an on-device A/B test.

**The four hard requirements (all mandatory):**
1. **`path` MUST be absolute:** `/mnt/ext1/applications/<name>.app`. `D:` root = `/mnt/ext1`.
   A relative path (`applications/<name>.app`) → app FAILS TO LAUNCH. ← the main bug.
2. **`U_<name>` key MUST match the `.app` filename.** `U_othello` → `othello.app`. Versioned
   names break it: `U_irad` → `irad_v2.app` fails. Copy the binary to `irad.app` so it matches.
3. **`icon`/`focused_icon` MUST be plain strings, NOT `{ "path": ... }` objects.** Object form →
   launches but NO icon shows (blank/white). Missing icon file → broken "image-missing" placeholder.
4. **NO `param` field.** The firmware rejects it: `Unsupported application field
   [applications.U_x.param]`.

**Icons:** 8-bit BMP, ≤128 wide × 106 tall, in `D:\applications\icons\`, two per app:
`<name>.bmp` and `<name>_f.bmp`. Generator: `scratchpad/mkicon/` (writes 8-bit + focused).

**Correct entry (verbatim):**
```json
"U_othello": {
    "path": "/mnt/ext1/applications/othello.app",
    "title": "Othello",
    "icon": "/mnt/ext1/applications/icons/othello.bmp",
    "focused_icon": "/mnt/ext1/applications/icons/othello_f.bmp"
}
```
Then add the string `"U_othello"` to a `view.groups[].apps` array (e.g. `@Games`).
Built-in `PB_*` games live ONLY in the group list (no `path`) — firmware launches them; don't touch.

**Deploy:** the whole correct applications block is generated by `scratchpad/vjfinal/main.go`
(edit the `games` list inside it). Steps: back up the live view.json → ensure name-matching
`.app` files + all `<name>.bmp`/`<name>_f.bmp` icons exist → run vjfinal → validate JSON
(`scratchpad/jsoncheck`). **The device re-reads view.json on USB DISCONNECT — no reboot needed.**
`ASCII-safe titles` are safest (we used "Einsteins Gata" without å).

⚠️ view.json can be **factory-reset** by a firmware update (all `U_` entries vanish, `.app` files
stay). If apps lose their icons/names, re-check the live file and re-run vjfinal.

---

## 9. What's on the device now

`D:\applications\` (clean names, no version suffixes) — 14 games + KOReader + Läsordning:
`irad, mastermind, blackbox, einstein, othello, nonogram, hex, bullscows, sudoku, lightsout,
nim, anagram, bagels, jotto, koreader, lasordning` (all `.app`). All registered in `view.json`
under **@Games** with 8-bit BMP icons in `applications/icons/`, using the §8 recipe (absolute
paths, string icons, name-matched). All 14 games have a splash + rules screen (§10).
Mastermind includes the Knuth "device guesses your code" mode. Latest good view.json backup on
device: `view.json.bak_final`. Builder for the whole view.json block: `scratchpad/vjfinal/main.go`.

## 10. Rules screen + splash screen — every game gets both (standard now)

As of the 2026-07 pass, **all games have a splash screen (shown first) and a rules screen
(reached from the menu)**. Any NEW game should include both from the start, in the same style,
so the library stays consistent. Both are pure add-ons — they never touch game logic.

### Screen-state plumbing (same for every game)
Add two states to the `screen` enum. **`screenSplash` must be the FIRST/initial state** (its
zero value), so it shows on launch; the menu is no longer the initial screen:
```go
const (
    screenSplash screen = iota   // shown on launch; tap → menu
    screenMenu
    screenGame                   // (or screenPlay/screenResult for einstein)
    screenRules                  // reached from a "Regler" button on the menu
)
```
- In `Init()` (or the constructor), set the initial screen to `screenSplash`, not `screenMenu`.
- In `Draw()`, add `case screenSplash:` → `DrawSplash(...)` and `case screenRules:` → `DrawRules(...)`, both followed by `FullUpdate()`.
- In `handleTap`, `case screenSplash:` advances to `screenMenu` on ANY tap. `case screenRules:` returns to the menu when the back button is hit.
- In `Key`, let hardware **Back** from `screenRules` (and `screenGame`) return to the menu.
- On the menu, draw a **"Regler"** button and route a tap on it to `screenRules`.

### Rules screen: `DrawRules(screen, fonts, title, paragraphs) image.Rectangle`
Title (bold ~56) centered at top, then word-wrapped body paragraphs, then a centered
**"Tillbaka"** button; return the button rect so `handleTap` can test it. Text is a
`[]string` of paragraphs; wrap each to the screen width with this helper (avoids importing
`strings`, keeps the wrap measured by the real font):
```go
func wrapText(s string, maxW int) []string { /* greedy word-wrap using ink.StringWidth */ }
func splitWords(s string) []string          { /* split on ' ' */ }
```
Body font ~34, `margin 60`, `lineH 46`, `paraGap 24`, `maxW = screen.X - 2*margin`.
Even Black Box's 8-paragraph rules fit above the button at 1072×1448 — verified in the emulator.
Keep Swedish rules text; **write the FULL rules here** (this replaced the old cramped 3-line
menu help). Black Box especially: explain H (hit), R (reflection), the numbered detour pairs,
and how an atom deflects vs. absorbs a ray.

### Splash screen: `DrawSplash(screen, fonts, title, motif)` — chess-app style
The built-in chess app opens with 4 big pieces in simple line graphics; we mirror that per game.
Layout: big bold title (~72–80) at `screen.Y/6`, a centered square motif box
(`side = screen.X*3/5`, centered), and a grey **"Tryck för att börja"** hint at `screen.Y*5/6`.
`motif` is a `func(box image.Rectangle)` that draws the game's own icon-like line-art:

| Game        | Motif                                                             |
|-------------|------------------------------------------------------------------|
| Othello     | 4 discs (2 solid, 2 ring) in a 2×2                                |
| Hex         | 7-hex honeycomb flower + 2 stones (1 solid, 1 ring)              |
| Nonogram    | small grid whose filled cells form a heart                       |
| Bulls & Cows| digit boxes "1 2 3 4" + a bull/cow marker row                    |
| I rad       | the 4 player marks X O △ □ in a 2×2                              |
| Black Box   | 5×5 grid with one atom + a ray that enters left and deflects up   |
| Mastermind  | row of 4 code pegs with distinct patterns + feedback-peg row      |
| Einstein    | 3×3 logic grid with a few O / X deduction marks                   |

Reuse each game's existing draw helpers where possible (disc/ring fills, `drawHexOutline`,
grid drawing). Keep it monochrome line/fill art — no text inside the motif except where the
game's pieces literally are glyphs (X O △ □, digits, O/X). **Remember §0a: in the emulator the
title/marks show as `?`; that's the stub font, correct on-device.**

### Where the code lives per game (they have different structures)
- **Single `package main` + `ui.go`** (othello, nonogram, hex, bullscows, blackbox): append
  `DrawSplash`/`DrawRules` + `wrapText`/`splitWords` + `drawSplashMotif` to `ui.go`.
- **Separate `ui` package** (irad): put `DrawSplash`/`DrawRules` in the `ui` package (e.g.
  `ui/splash.go`, `ui/rules.go`), export `SplashMotif`; the `MenuAction` struct gains an
  `OpenRules bool` so a menu tap can request the rules screen.
- **`App` with `button` type** (mastermind): add `drawSplash`/`drawRules` methods; the splash/
  rules back buttons are `button` values stored on the struct and tested with `.hit(p)`.
- **`Game` with `btnHits`** (einstein): add `drawSplash`/`drawRules` methods; the rules "back"
  and menu "Regler" are `btnHit{action:"…"}` entries handled in the tap loop. `screenSplash`
  is the new initial `scr`; a "Regler" button sits under "Starta" on the menu.

### Stub gotchas hit this pass
- The `inkstub`/`inkrender` stubs must expose **every** ink symbol a game references, or the
  cgo-free build fails. This pass we had to add `ink.Pad(r, n)` and `ink.Exit()` to both stubs
  (mastermind uses them). When a `go build`/`vet` fails with `undefined: ink.X`, add `X` to
  both stubs to mirror the real API — don't change the game.

---

## 11. Known follow-ups / ideas

- ✅ In-app rules screen — DONE for all 14 games (§10), including full Black Box ray rules.
- ✅ Splash screens — DONE for all 14 games (§10).
- ✅ Mastermind **Knuth solver mode** ("device guesses your code") — DONE (gated to classic 4/6).
- More variants / new games. Each new game: copy an existing module as a template, keep logic
  ink-free, reuse the Pointer+font-cache pattern, **add a splash + rules screen (§10)**, verify
  in the emulator, build under a fresh name. Ideas not yet built: "Vem är det?" attribute
  deduction, laser-maze mirrors (Black Box cousin), more Sudoku/Nonogram sizes, MCTS Hex AI.

## 12. Survey of community PocketBook game repos (2026-07)

Studied three community repos to check for build/deploy techniques we'd missed:
[SteffenBauer/PocketPuzzles](https://github.com/SteffenBauer/PocketPuzzles) (MIT — a port of
Simon Tatham's Portable Puzzle Collection), [OliverHaag/pb-mahjong](https://github.com/OliverHaag/pb-mahjong)
(GPLv3), [JuanJakobo/Pocketbook-Tic-Tac-Toe](https://github.com/JuanJakobo/Pocketbook-Tic-Tac-Toe)
(GPLv3, archived).

**Toolchain — not reusable.** All three link the **official PocketBook C SDK** (`inkview.h`,
`InkViewMain()`, real `libinkview`) via Makefile/CMake + the Obreey ARM cross-compiler. This is a
different linkage model from our Go+cgo-stub approach (we stub the SDK out; they link the real
thing) — their source can't be merged into our pipeline, and porting any of their games means a
full Go reimplementation, not code reuse. Their compiled `.app` binaries *would* run as-is on our
device (same firmware/platform) if we ever wanted a binary drop-in, but that's not the plan.

**Confirms what we already knew:** all three use `EVT_POINTERDOWN`/`EVT_POINTERUP` for taps (not
touch events) — independent confirmation of the Pointer-not-Touch gotcha (§4).

**No repo uses view.json or any manifest.** All three just drop a `.app` into `/applications` and
rely on firmware auto-discovery — appears as `@<binary-name>`, no custom icon. This confirms
view.json (§8) is only *needed* when you want a custom icon + grouped placement (our `@Games`
folder); plain drop-in is the firmware's simpler default fallback.

**New trick worth knowing (not yet adopted):** pb-mahjong's README shows a **separate mechanism**
for a friendly launcher name — add a line like `@pb-mahjong=Mahjong Solitaire` to the device's
`system/language/en.txt`. This only renames the label (no icon), and is independent of view.json.
Could be a useful *fallback* for readable app names if view.json ever gets factory-reset by a
firmware update, since editing the language file wouldn't be wiped the same way (unverified —
would need testing). Not implemented; just noting the option.

**Other patterns seen, not currently relevant:** `pbres` compiles BMP icons into the binary as C
arrays at build time (for in-game art, not launcher icons — doesn't change our icon recipe in §8).
pb-mahjong's CMake `buildpackage` target tars up the install tree and supports rsync/scp deploy to
a networked device as an alternative to manual USB copy — could inspire a scripted deploy step if
we ever confirm the device exposes SSH/network shares, but untested.

**Game ideas surfaced:** PocketPuzzles ports ~52 Simon Tatham puzzles. Cross-checked against our
lineup and `SPEC_NEXT_GAMES.md` — the overlaps (Guess=Mastermind, Blackbox, Solo=Sudoku,
Pattern=Nonogram) we already have; Akari (Light Up) and Slitherlink (both in `SPEC_NEXT_GAMES.md`
already) and Bridges/Hashiwokakero (also already spec'd as group 4) are exactly the strongest picks
from that list, so no new spec needed — the existing `SPEC_NEXT_GAMES.md` priorities are validated
by this survey. Also notable: PocketPuzzles explicitly **excluded Sokoban/Group/Slide for eInk
rendering/animation problems** — good confirmation to keep avoiding drag/animation-heavy puzzle
types on this device (Untangle, Inertia, Cube, Twiddle) in favor of pure tap-based grid logic.
