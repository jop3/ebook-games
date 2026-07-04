# En Studie i Grått — a tap-driven mystery (scaffold)

**En Studie i Grått** ("A Study in Grey") is an original locked-room mystery for
the PocketBook Verse Pro, and the **second story on the Grottan adventure
engine**. It is the worked example from `../SPEC_TEXT_ADVENTURE.md` §10d, and it
exists to prove the engine-reuse thesis end to end:

> a themed story is **shared engine + one extension + authored data**, not a rewrite.

## What is reused vs. new

| | Source | Notes |
|---|---|---|
| **Engine (reused verbatim)** | `story/model.go`, `story/engine.go`, `story/save.go` | **Byte-for-byte identical** to `grottan/story/*` (same sha256). Data model, State, New/Describe/Move/Act/IsDark/save — all shared, unchanged. |
| **The one extension** | `story/notebook.go` | The Notebook / deduction system (spec §10b): clue collection + a `combine` table, built on the `Clues`/`Deductions` fields that were designed into `State` from the start. |
| **Authored data** | `story/storydata.go` | ~6 hand-written rooms + clue objects of an original fog-bound case (fresh prose, no transcription). |
| **Themed chrome** | `main.go`, `ui.go`, `vignettes.go` | A Notebook screen instead of a map; the "Blocket" button; splash motif (a magnifying glass over a boot-print); Swedish "Fallet" menu/rules; fog/gaslight vignettes drawn on the **shared `draw_toolkit.go`** (also copied verbatim). |

The shared engine still carries a few cave-only hooks (the lamp, grate, and
XYZZY magic word). Rather than fork the engine, this story defines those symbols
as **inert sentinels** in `storydata.go` (negative ids that never match, all
rooms lit so the darkness path never runs). A future cleanup could lift those
hooks into per-story data so the engine is fully story-agnostic; the point of
the scaffold is that it already runs on the identical engine source.

## The loop

Examine objects (tap **Undersök** then a **föremål**, or just tap the object) to
record **ledtrådar** into the notebook. Open **Blocket** and tap **two clues**
to draw a **slutsats** — matching pairs yield a deduction, others give "inget
samband". The case has 3 deductions (how the killer entered, the motive, the
timing).

## Status — this is a scaffold

Working: engine reuse, the full examine→notebook→combine→deduction loop, themed
chrome + vignettes, save/restore, play-tested with screenshots.

**TODO to become a full game** (spec §10d): the accusation endgame
(culprit / method / motive screen); more rooms and character-interview nodes;
clue-combining that *unlocks new areas/testimony* rather than only recording
text; a Swedish narration pass (prose is currently English, matching the repo
pattern); and the launcher `.app` build + `view.json` entry (needs the
Windows/WSL + Docker toolchain — see `../grottan/README.md`).

## Verify (no device needed)

```bash
cd studie && go test ./story/           # engine + notebook unit tests
playtest/play.sh studie                 # full UI playthrough + screen renders
```
