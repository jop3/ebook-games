# Feasibility evaluation — next-game candidates

Evaluation of ten proposed games against the reality of this repo and target device.
Companion to `SPEC_NEXT_GAMES.md` (which specs the previously-chosen batch) — this doc
grades a **new** shortlist and recommends what to build.

**Verdict at a glance**

| Game | Fit | Effort | Verdict |
|---|---|---|---|
| **YINSH** | ★★★★☆ | Medium–High | ✅ **Build** — distinctive deep abstract, ideal e-ink visuals; AI quality is the only risk |
| **Hasami shogi** | ★★★★★ | Low–Medium | ✅ **Build** — cheap (fork `othello`), distinct sandwich-capture mechanic |
| **Sushi Go** | ★★★★☆ | Medium | ✅ **Build** — the library's first *card/drafting* game; good genre gap-fill |
| **Go** | ★★★★☆ (2P) | Medium (+High for AI) | ✅ **Build 2-player / 9×9**; ship a weak 9×9 AI, don't promise a strong one |
| **Shong** | ★★★★★ | Low–Medium | ✅ **Build** — chess-like on a tiny 4×6 board; distinct shapes, and the small board makes a *strong* AI easy |
| **Donuts** ("Insert") | ★★★★☆ | Medium | ✅ **Build** — genuinely novel 2-player abstract (direction-forcing + custodial capture + connect-5) |
| **Desdemona** | ★★☆☆☆ | Low | ❌ **Skip as standalone** — its most-popular form is just renamed Reversi (= our Othello). Add an Othello *variant mode* instead |
| **Shogi (full)** | ★★☆☆☆ | High | ❌ **Skip** — AI + kanji rendering + drops UI too costly; Hasami covers the itch |
| **Gomoku** | — | — | ❌ **Already shipped** — `irad` includes it ("fem i rad / Gomoku") |
| **Cyberbox** | ★★☆☆☆ | Medium | ❌ **Skip** — Sokoban family; e-ink guidance in this repo explicitly avoids it |
| **Bejeweled** | ★☆☆☆☆ | Medium | ❌ **Skip** — cascade animation + color + speed all fight e-ink |

---

## Rubric (why these axes)

Feasibility here is dominated by the **device**, not by raw coding difficulty. Every game
is scored on:

1. **Rules/logic** — implementing correct rules in pure `game/` Go (unit-testable, cgo-free).
2. **AI** — does it need an opponent, and how hard is a *decent* one on a 32-bit ARM e-reader?
3. **Input fit** — the device is **tap-only**. Drag / fast gestures are out (guide §4, §5a).
4. **Render fit** — **greyscale + slow e-ink refresh, no animation** (guide §5, §12). Pieces
   must be distinguishable by *shape/pattern*, never color.
5. **Content** — level packs? a uniqueness-verifying generator? bespoke tile/card art?
6. **Reuse** — can we fork an existing module (`othello`, `hex`, `sudoku`, `nonogram`)?
7. **Redundancy** — do we already have this mechanic in the library?

The repo's own hard-won lessons that decide several verdicts below:
- PocketPuzzles (a mature port to this exact device class) **excluded Sokoban/slide/animation
  games** for "eInk rendering/animation problems" (guide §12). → hits **Cyberbox**.
- Non-ASCII glyphs (`◂ ▸`, etc.) **render as broken boxes on-device** (guide §5a). → hits full
  **Shogi** (kanji pieces).
- Color is *always* substituted by pattern/texture on this hardware. → hits **Bejeweled**.

---

## ✅ Build

### YINSH  (Kris Burm, GIPF project) — deep 2-player abstract
Board = truncated 6-point star, 85 intersections on a triangular grid. Each player has 5 rings;
moving a ring leaves a marker and flips every marker jumped (Othello-style); make a row of 5 of
your color → remove a ring; remove 3 rings to win.

- **Logic:** moderate–high but fully deterministic and pure-Go. Ring movement along 3 axes,
  marker-flip along the jumped segment (conceptually the same raycasting as `othello`), row-of-5
  detection on 3 axes, ring removal. Tractable.
- **AI:** *the* risk. YINSH has a large branching factor and deep strategy. A **strong** AI is a
  research effort; a **modest heuristic minimax (depth 2–3)** with a decent eval is achievable and
  gives an okay opponent — pair it with 2-player hot-seat. Reuse the `othello` AI scaffolding.
- **Input:** tap ring → tap destination; flips resolve automatically. Excellent tap fit.
- **Render:** rings = outline circles, markers = solid disc (black) vs outline/white disc — a
  literal black/white game, **ideal for greyscale**. Triangular/hex board geometry has precedent
  in `hex`.
- **Reuse:** board geometry from `hex`; flip + AI patterns from `othello`.
- **Verdict:** highest-value new *strategy* title. It's genuinely distinctive (nothing in the
  library plays like it) and looks great on e-ink. Ship with a modest AI + hot-seat; label the AI
  honestly. **Medium–High effort.**

### Hasami shogi — capture + line-building on a 9×9
Pawns only; move like a rook; **capture by sandwiching** (custody flanking); win by an unbroken
line of 5 (variant: capture all-but-one). *Not* full Shogi.

- **Logic:** simple, pure-Go, easy to unit-test.
- **AI:** moderate. Minimax with a simple eval; rook-style moves give moderate branching but it's
  well within reach. Fork the `othello` turn/AI structure.
- **Input/Render:** tap piece → tap destination; two stone types (black/white disc or X/O).
  Trivial greyscale fit.
- **Reuse:** **very high** — copy `othello` and swap the rules.
- **Redundancy note:** the 5-in-a-row *goal* overlaps thematically with `irad`, but the
  **movement + sandwich-capture** mechanic is entirely different and not in the library.
- **Verdict:** best effort-to-payoff ratio on the list. **Low–Medium effort.** Build it.

### Sushi Go — card drafting (genre gap-fill)
Draft 1 card from your hand, pass the hand, repeat; score sets over 3 rounds (maki majority,
sashimi triples, tempura pairs, scaling dumplings, nigiri+wasabi, pudding, chopsticks). 2–5
players.

- **Logic:** moderate scoring rules; pure-Go and very unit-testable (each scoring category is an
  independent function — same discipline as the puzzle validators).
- **AI:** opponents need a **drafting AI** — a greedy expected-value heuristic with light
  lookahead is enough for a fun opponent. Moderate.
- **Input:** tap a card to draft. No animation needed (turn-based reveal). Great tap fit.
- **Render:** ~8 card types → ~8 simple greyscale icons + counts. Legible icons are doable
  (this is real but bounded art work, like the splash motifs).
- **Multiplayer shape:** hidden simultaneous hands make hot-seat awkward → design as **1 human vs
  N AI**.
- **Verdict:** the whole library is abstract/logic games; this would be the **first card game**,
  which is exactly the kind of diversification worth adding. **Medium effort** (icons + drafting
  AI + scoring). Build it.

### Go — iconic, and the *best* possible e-ink fit
- **Render/Input:** black and white stones on a line grid — this is **the** perfect greyscale
  game; nothing to translate. Tap an intersection. Flawless fit.
- **Logic:** placement + capture (liberties, ko) is straightforward. **Territory scoring +
  dead-stone detection** is the fiddly part — the standard pragmatic answer is *manual dead-stone
  marking at game end*, which is simple and tap-friendly.
- **AI:** the famous hard problem. A **strong** Go AI is out of scope (MCTS is CPU-heavy for this
  ARM chip). A **weak 9×9** heuristic/light-playout AI is playable-but-weak.
- **Verdict:** **build it as 2-player hot-seat + 9×9**, with an optional weak AI clearly labeled.
  Iconic, gorgeous on e-ink, and the 2-player path is low-risk. Don't over-invest in AI strength.
  **Medium effort** (much higher only if you chase a strong AI — don't).

---

### Shong — chess-like duel on a tiny board  *(identity now confirmed)*
Free abstract by Higher Plain Games. **4-wide × 6-tall** board; four piece types — Triangle
(diagonal), Square (orthogonal), X (omnidirectional), King (one step, alternating tri/square).
Non-king pieces start with a 1-square "short move" and flip to a 2-square "long move" after their
first move (an eye symbol marks the state). No jumping; clear line of sight required. **Win by
capturing the enemy King *or* walking your King to the far edge.**

- **Logic:** simple and pure-Go. The only wrinkle is the per-piece short/long toggle — one bool of
  state per piece. Easy to unit-test.
- **AI:** *this is the standout.* The board is only 24 cells, so alpha-beta minimax searches deep
  cheaply — you can ship a genuinely **strong** AI (the three-difficulty ladder the original has),
  unlike YINSH/Go where AI is the risk. Fork `othello`'s AI scaffolding.
- **Input:** tap piece → tap destination; a small mark shows each piece's short/long state. Ideal
  tap fit.
- **Render:** the four types map to **Triangle / Square / X / King** — the library *already draws*
  △ □ X as player marks in `irad`, so these glyphs are proven on-device. The original's color
  shifts are cosmetic balance feedback → **drop them**, they carry no rules.
- **Reuse:** `othello` (turn loop + AI), `irad` (△□X glyphs).
- **Verdict:** upgraded from "clarify" to a **strong Build** now that it's identified. Small board
  + distinct shapes + easy strong AI = low-risk, high-polish. **Low–Medium effort.**

### Donuts — direction-forcing abstract with custodial capture  *("Insert" = its capture rule)*
Funforge, 2021 (BGG 341358). Four 3×3 tiles shuffled into a **6×6** grid; every cell carries a
line (vertical / horizontal / diagonal). Place a ring; **the line in that cell dictates the
direction your opponent must play next** (if that line is full, they play anywhere). **Custodial
capture:** *inserting* a ring so it flanks opponent rings (`O_O`, and the gapped `OXX_O` form)
flips them to your color. **Win** instantly on 5-in-a-row (any direction); otherwise the largest
orthogonally-connected group wins. *("Insert" in the shortlist was describing this capture — it's
one game, not two.)*

- **Logic:** moderate, pure-Go. The direction-constraint lookup, the custodial flip (an Othello
  cousin, plus the gapped `OXX_O` variant), connect-5 detection, and the largest-group tiebreak
  are each an independent, unit-testable function.
- **AI:** 6×6 with place-and-flip → moderate branching; alpha-beta at a decent depth is
  comfortable. Achievable, reusing `othello`'s search.
- **Input/Render:** tap an empty cell to place. Two piece states → filled disc vs. outline ring
  (greyscale-clean). Cell lines render as simple `│ ─ ╱ ╲` line-art. Static board, no animation.
- **Reuse:** `othello` (flip + AI + board); the direction-forcing rule is the novel part.
- **Novelty:** high — nothing in the library forces the opponent's move direction like this.
- **Note:** it's a *commercial* game — rules aren't copyrightable, but ship original art and pick
  a neutral/Swedish name (the library renames anyway, e.g. "Einsteins Gåta"), as with any port.
- **Verdict:** **Build.** Distinct mechanic, clean e-ink/tap fit, good `othello` reuse.
  **Medium effort.**

---

## ❌ Skip as a standalone — Desdemona

Researched the name to pick "the most popular ruleset": there **isn't a single famous distinctive
one.** The most-implemented thing called Desdemona (the top result, `stig/Desdemona` for macOS) is
just a **plain Reversi/Othello** game under a different name. The handful of BGG board games named
"Desdemona" are obscure one-offs, each with different rules and none widely played.

- So building "Desdemona" as its own app would either **duplicate our existing `othello`** (if we
  take the popular = plain-Reversi reading) or force us to arbitrarily pick an obscure variant.
  Either way the payoff is poor.
- **Better use of the same effort — an Othello *variant mode*** toggled on the `othello` menu, with
  a genuinely different, well-known rule. Cleanest option: **"Anti-Othello" / Reversed** — identical
  play but the player with the **fewest** pieces at the end wins (a one-line win-condition flip that
  plays completely differently and is a recognised Reversi variant). This adds real variety for
  near-zero cost and no new app, icon, or art.
- **Verdict:** don't ship a separate Desdemona; add the Anti-Othello mode to `othello` if you want
  the variety.

---

## ❌ Skip (with reasons)

### Shogi (full)
- **AI is the killer:** drops (captured pieces re-enter) blow up the state space beyond chess; a
  competent AI is a major effort and even a weak one is heavy on this ARM chip.
- **Rendering:** pieces are **kanji**, and the guide (§5a) documents that non-ASCII glyphs render
  as **broken boxes on-device** — you'd need bespoke bitmap piece art or romaji abbreviations at
  small cell size. Extra burden.
- **Plus** a drops UI (select-from-hand, place) on top.
- **Conclusion:** disproportionate cost for the library. **Hasami shogi delivers the "shogi"
  flavor at a fraction of the cost** — build that instead.

### Gomoku
- **Already in the library.** `irad` is explicitly "X-in-a-row (tre/fyra/fem i rad, **Gomoku**)".
  A separate app would be redundant. (A dedicated *Renju* with pro forbidden-move rules + a strong
  AI is the only differentiator, and it's low value.)

### Cyberbox
- **Sokoban family** (push blocks, special slider/pusher/zapper types, reach the exit, 16 rooms).
  It's turn-based so *technically* possible — but this is exactly the class the repo has
  deliberately steered away from: guide §12 records PocketPuzzles **excluding Sokoban/slide games
  for e-ink rendering/animation problems**, and pushing chains of blocks forces frequent
  full-screen redraws on a slow, ghosting panel.
- Also needs a **hand-authored level pack** (the originals aren't ours to ship).
- **Conclusion:** against documented guidance and a weak movement-puzzle fit. Skip.

### Bejeweled
- **Match-3 swap with cascades**, usually timed/scored. The appeal *is* the falling-gem cascade
  animation and speed — both fight e-ink's slow refresh and ghosting head-on.
- Traditionally **color-coded** gems → must map to ~7 greyscale symbols (doable, but the animation
  problem is the dealbreaker, not the color one).
- A turn-based, untimed match-3 is buildable but loses the essence.
- **Conclusion:** wrong genre for this hardware. Skip.

---

## Recommended order

Low-risk first — the small-board 2-player abstracts fork `othello` and get a *strong* AI cheaply;
YINSH and Go carry AI risk and go later.

1. **Hasami shogi** — cheapest win, forks `othello`, distinct mechanic.
2. **Shong** — tiny 4×6 board → strong AI is trivial; distinct chess-like duel; reuses `irad`'s △□X.
3. **Donuts** — novel direction-forcing + custodial capture; forks `othello`'s flip/AI.
4. **Sushi Go** — first card game; broadens the library's genre mix.
5. **YINSH** — highest-value new strategy title; ideal e-ink visuals (budget for AI tuning).
6. **Go** (2-player + 9×9) — iconic, perfect greyscale fit; weak AI optional.
7. *(near-zero-cost extra)* an **"Anti-Othello" variant mode** on the existing `othello` — the
   worthwhile substitute for a standalone Desdemona.

Every build still follows the standard §0 setup + splash/rules screens + `play_test.go` from
`SPEC_NEXT_GAMES.md` and the guide.

**No open questions remain** — all nine titles are identified and specced above. (Desdemona
resolved to "renamed Reversi," so it's folded into `othello` as an optional mode rather than a new
app.)
