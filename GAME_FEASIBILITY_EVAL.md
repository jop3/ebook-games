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
| **Desdemona** | ★★★☆☆ | Low | 🟡 **Maybe** — fork `othello`; low novelty. Confirm exact ruleset first |
| **Shong** | ？ | Medium | 🟡 **Clarify** — best guess = mahjong/Shisen tile-match (feasible); identity unconfirmed |
| **Insert / donuts** | ？ | ？ | 🟡 **Clarify** — best guess = match-3/merge (poor e-ink fit); identity unconfirmed |
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

## 🟡 Maybe / needs clarification

### Desdemona — an Othello/Reversi variant
"Desdemona" is a Reversi twist (the common versions let capture lines **wrap around edges or turn
corners**, vs. straight-line-only Othello). It is an **Othello engine with a modified flip rule**.

- **Effort:** low — fork `othello`, change the flip raycasting, reuse board/UI/AI (retune eval).
- **Caveat:** **low novelty** — we already ship Othello; this is an incremental variant. Cheapest
  option on the list, but adds a twist rather than a new genre. Could even live as a "variant
  mode" inside `othello` instead of a standalone app.
- **Action:** confirm the exact Desdemona ruleset you mean before building (wrap vs. corner-turn
  vs. board shape) — the flip logic depends on it.

### Shong — identity unconfirmed
Not resolvable to one authoritative game. **Best guess: a mahjong-solitaire / Shisen-Sho
tile-matching game.** Under that reading:

- **Fit:** static layout, tap-two-tiles-to-match → good e-ink/tap fit; no AI (solitaire).
- **Cost:** the burden is **distinct greyscale tile faces** (mahjong has ~34–42 designs) legible
  at cell size, plus a **solvable-deal generator** (guarantee winnable). Medium effort. Shisen-Sho
  (flat path-connect matching) is geometrically simpler than layered "turtle" mahjong — prefer it
  if this is the target.
- **Action:** tell me which "Shong" you mean and I'll firm up the estimate.

### Insert / donuts — identity unconfirmed
Searches map "donuts" to generic **match-3 / merge / connect-3** mobile games, and "Insert" is
unclear (possibly a Connect-Four-style column-insert game).

- If **match-3/merge**: poor fit — same animation/color problems as Bejeweled below → **Skip**.
- If **Insert = a Connect-4 column-drop abstract**: feasible, but we already cover Connect Four as
  a mode in `irad`, so **low novelty**.
- **Action:** clarify which game(s) these are; as I read them today, neither is a strong pick.

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

1. **Hasami shogi** — cheapest win, forks `othello`, distinct mechanic.
2. **YINSH** — highest-value new strategy title; ideal e-ink visuals (budget for AI tuning).
3. **Sushi Go** — first card game; broadens the library's genre mix.
4. **Go** (2-player + 9×9) — iconic, perfect greyscale fit; weak AI optional.
5. *(cheap extra)* **Desdemona** as an `othello` variant — once its ruleset is pinned down.

Every build still follows the standard §0 setup + splash/rules screens + `play_test.go` from
`SPEC_NEXT_GAMES.md` and the guide.

**Open questions for you:** (a) which exact **Desdemona** ruleset? (b) what are **Shong** and
**Insert / donuts** specifically — my identifications above are best-guesses and change the
verdict for those three.
