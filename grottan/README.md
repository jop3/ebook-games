# Grottan — a tap-driven Colossal Cave for the PocketBook Verse Pro

**Grottan** ("the cave") is a text-adventure port of *Colossal Cave Adventure*
for the PB634, replacing the classic typed parser with a **tap verb + tap noun**
interface (Scott-Adams two-word model). Tap an exit to move; arm a verb then tap
a noun to act; discovered magic words appear under **Säg…**. The game auto-saves.

This is **Phase 1** (see `../SPEC_TEXT_ADVENTURE.md` §2): the data-driven engine,
the tap UI, save/restore, splash + rules, and the surface-world + first-cave
subset of the game (well house → grate → debris room → Hall of Mists, with the
lamp, keys, grate, cage/bird/rod, gold, and the XYZZY magic word). No dwarves,
RNG, maze, or scoring yet — those are Phases 2–3.

## Layout

```
grottan/
  main.go                  ink.App: event loop, screens, tap dispatch, autosave
  ui.go                    drawing (imports ink): layout, splash, rules, menu
  story/                   PURE engine — no ink import, unit-tested cgo-free
    model.go               data types + State + New()
    engine.go              IsDark/Describe/Exits/Move/Act/... (pure functions)
    save.go                gob save/restore, version-guarded
    storydata_gen.go       GENERATED from Open Adventure's adventure.yaml
    engine_test.go         unit tests (scripted walk, take/drop, save round-trip)
  play_test.go             //go:build playtest — screen renders + UI playthrough
  third_party/inkview/     vendored SDK (cgo; used only for the device build)
  THIRDPARTY.md            Open Adventure BSD-2 license + attribution
```

The world data is **generated**, not hand-written: `scratchpad/advgen/` ingests
`adventure.yaml` from a local Open Adventure checkout, keeps the Phase-1
allow-list, rewrites travel that leaves the subset into "you can't go that way"
messages, and emits `story/storydata_gen.go`. Regenerate when widening coverage:

```bash
go run ./scratchpad/advgen <path-to>/adventure.yaml grottan/story/storydata_gen.go
```

## Verify (no device needed)

```bash
# pure engine unit tests
cd grottan && go test ./story/

# full UI playthrough + every screen rendered to PNG (uses playtest/inkemu)
PLAYTEST_SHOTS=$PWD/playtest/_shots playtest/play.sh grottan
```

The play suite drives the real tap UI (splash → menu → new game → well house →
take lamp/keys → unlock grate → descend), asserts state at each step, renders
splash/menu/rules/room/dark-room/worst-case/scrolled/say-popup screenshots, and
fails if any text overflows the real 1340px drawable height (guide §5).

## Build the device .app + install  (needs the Windows/WSL + Docker SDK toolchain)

These steps require the cross-compile toolchain from `POCKETBOOK_GAMEDEV_GUIDE.md`
§7–§8 and are **not** runnable in a headless/web container — hand them off to the
dev machine:

1. **Build the ARM binary** (guide §7):
   ```bash
   echo noviso | sudo -S -p "" docker run --rm \
     -v "/mnt/c/github/Ny mapp/grottan:/app" -w /app \
     sunsung/pocketbook-go-sdk:latest build -o grottan.app .
   file grottan.app          # must say: ELF 32-bit LSB executable, ARM, EABI5
   ```
   (`go build` ignores `*_test.go`, and `play_test.go` is `//go:build playtest`,
   so neither the play tests nor the emulator touch the device build.)

2. **Icons**: make `grottan.bmp` + `grottan_f.bmp` (8-bit BMP, ≤128×106) from the
   cave-mouth/grate motif via `scratchpad/mkicon/`, into `D:\applications\icons\`.

3. **view.json @Games entry** (guide §8 recipe — absolute path, string-form
   icons, key `U_grottan` name-matched to `grottan.app`, no `param`):
   ```json
   "U_grottan": {
       "path": "/mnt/ext1/applications/grottan.app",
       "title": "Grottan",
       "icon": "/mnt/ext1/applications/icons/grottan.bmp",
       "focused_icon": "/mnt/ext1/applications/icons/grottan_f.bmp"
   }
   ```
   Add `"U_grottan"` to the `@Games` group and regenerate via `scratchpad/vjfinal`.

4. Copy `grottan.app` to `D:\applications\`, `sync`, eject; the device re-reads
   view.json on USB disconnect.
