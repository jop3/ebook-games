# En Studie i Grått — a tap-driven mystery (playable start to finish)

**En Studie i Grått** ("A Study in Grey") is an original locked-room mystery for
the PocketBook Verse Pro, and the **second story on the Grottan adventure
engine**. It is the worked example from `../SPEC_TEXT_ADVENTURE.md` §10d — a
**complete, winnable case**, and a proof of the engine-reuse thesis:

> a themed story is **shared engine + one extension + authored data**, not a rewrite.

## The case, start to finish

A man is found dead in a bolted study, no wound, a scent of bitter almonds. You
have your consulting-rooms as a hub, a foggy street, the lodging-house (hall,
study, bedroom, back yard), and — once you know to look — a chemist's shop and a
cabman to question. Gather **nine clues** (physical evidence + three interviews),
draw the **four deductions** (one, *the poison*, unlocks the chemist), then make
the **accusation**: culprit, method, motive. A wrong pillar or an unproven charge
is refused and named; the correct, fully-supported charge wins and plays the
closing scene. It is winnable in one sitting.

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

## Status — a complete, winnable case

Done: engine reuse; 8 rooms including three interview nodes; the full
examine→notebook→combine→deduction loop; a lead that **unlocks a new area** (the
poison deduction opens the chemist); the **accusation endgame** (culprit / method
/ motive, with wrong-pillar and unproven-charge handling) and a resolution scene;
themed chrome + fog/gaslight vignettes; save/restore. The whole case is played
start to finish in the play test, screen by screen.

**Remaining polish (optional):** a Swedish narration pass (prose is currently
English, matching the repo pattern; UI chrome is already Swedish); more suspects /
red-herring depth; and the launcher `.app` build + `view.json` entry, which need
the Windows/WSL + Docker toolchain — see `../grottan/README.md`.

## Verify (no device needed)

```bash
cd studie && go test ./story/           # engine + notebook unit tests
playtest/play.sh studie                 # full UI playthrough + screen renders
```
