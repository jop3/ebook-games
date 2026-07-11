# ebook-games

Games and small apps for the **PocketBook Verse Pro (PB634)** e-ink reader —
1072×1448 portrait, greyscale, 32-bit ARM. Built with the Go SDK
[dennwc/inkview](https://github.com/dennwc/inkview) (vendored per-project under
`third_party/inkview`).

Nearly **50 apps** so far: two-player abstracts with their own AI, a handful of
card and engine games, solo logic puzzles, deduction and word games, a
programmed-movement racer, two tap-driven text adventures, and a couple of
utilities. Every game has its own README with a description, the rules, and
screenshots — click any tile below.

See [POCKETBOOK_GAMEDEV_GUIDE.md](POCKETBOOK_GAMEDEV_GUIDE.md) for the full
toolchain writeup (WSL2 + Docker build, headless emulator, deploy, and every
trap hit along the way) before starting a new game.

---

## Abstract strategy — you vs the built-in AI

Two-player perfect-information games, each with an opponent AI (from simple
heuristics to alpha-beta and perfect play).

| | Game | About |
|:-:|---|---|
| [<img src="amazons/screenshots/amazons_splash.png" width="94">](amazons/) | **[Amazons](amazons/)** | Move like a chess queen, then fire an arrow that burns a square forever; the last player able to move wins. |
| [<img src="ataxx/screenshots/ataxx_splash.png" width="94">](ataxx/) | **[Ataxx](ataxx/)** | Clone into a neighbouring square or jump across the board, flipping every enemy piece you land beside. |
| [<img src="breakthrough/screenshots/breakthrough_splash.png" width="94">](breakthrough/) | **[Breakthrough](breakthrough/)** | Race a phalanx of pawns to the far rank; you may only capture diagonally. |
| [<img src="chomp/screenshots/chomp_splash.png" width="94">](chomp/) | **[Chomp](chomp/)** | Bite squares out of a grid; whoever is forced to eat the poisoned corner loses. Unbeatable AI. |
| [<img src="dominering/screenshots/dominering_splash.png" width="94">](dominering/) | **[Domineering](dominering/)** | One player lays vertical dominoes, the other horizontal; the first with no move loses. |
| [<img src="goban/screenshots/goban_splash.png" width="94">](goban/) | **[Go](goban/)** | Surround territory and capture stones on 9×9 / 13×13 / 19×19, with ko, area scoring, and a mark-dead phase. |
| [<img src="hasami/screenshots/hasami_splash.png" width="94">](hasami/) | **[Hasami shogi](hasami/)** | Leap over and custodially capture enemy pieces; win by captures or by forming a line. |
| [<img src="hertigen/screenshots/hertigen_splash.png" width="94">](hertigen/) | **[The Duke](hertigen/)** | Every tile moves by the pattern on its face and flips to a new pattern after moving; capture the Duke. |
| [<img src="hex/screenshots/hex_splash.png" width="94">](hex/) | **[Hex](hex/)** | Connect your two sides of the rhombus with an unbroken chain of stones; draws are impossible. |
| [<img src="hexa/screenshots/hexa_splash.png" width="94">](hexa/) | **[Hexa (Six)](hexa/)** | Place then move hexagonal tiles to form a line, triangle, or ring of your colour. |
| [<img src="hnefatafl/screenshots/hnefatafl_splash.png" width="94">](hnefatafl/) | **[Hnefatafl](hnefatafl/)** | Asymmetric Viking siege (Brandub 7×7): the king bolts for a corner while the attackers surround him. |
| [<img src="irad/screenshots/irad_splash.png" width="94">](irad/) | **[I rad](irad/)** | X-in-a-row — three, four, five, or Gomoku — for 1–4 players against the AI. |
| [<img src="isola/screenshots/isola_splash.png" width="94">](isola/) | **[Isola](isola/)** | Move your pawn, then destroy a square; strand your opponent with nowhere left to step. |
| [<img src="konane/screenshots/konane_splash.png" width="94">](konane/) | **[Kōnane](konane/)** | Hawaiian checkers — jump-capture along rows and columns; the first player unable to move loses. |
| [<img src="lgame/screenshots/lgame_splash.png" width="94">](lgame/) | **[L-Game](lgame/)** | De Bono's 4×4 duel: reposition your L-piece (and maybe a neutral disc) to leave the other L no move. |
| [<img src="munkar/screenshots/munkar_splash.png" width="94">](munkar/) | **[Munkar (Donuts)](munkar/)** | Direction-forcing placement with an inverted capture twist on a small board. |
| [<img src="murar/screenshots/murar_splash.png" width="94">](murar/) | **[Quoridor](murar/)** | Race your pawn to the far side while dropping walls to lengthen your opponent's path. |
| [<img src="nim/screenshots/nim_splash.png" width="94">](nim/) | **[Nim](nim/)** | Take objects from rows; the perfect-play (Sprague–Grundy) AI never slips. |
| [<img src="othello/screenshots/othello_splash.png" width="94">](othello/) | **[Othello / Reversi](othello/)** | Outflank to flip discs; includes an Anti-Othello ("Omvänd") fewest-discs variant. |
| [<img src="quarto/screenshots/quarto_splash.png" width="94">](quarto/) | **[Quarto!](quarto/)** | Your opponent hands you the piece you must place; make a line sharing any single attribute. |
| [<img src="ringar/screenshots/ringar_splash.png" width="94">](ringar/) | **[YINSH](ringar/)** | Move rings, flip the markers they leave behind, and remove your five-in-a-rows to win. |
| [<img src="shong/screenshots/shong_splash.png" width="94">](shong/) | **[Shong](shong/)** | A tight 4×6 duel against a genuinely strong AI. |
| [<img src="stadskarnan/screenshots/stadskarnan_splash.png" width="94">](stadskarnan/) | **[Cathedral](stadskarnan/)** | Place polyomino buildings to enclose and claim city territory. |
| [<img src="staplarna/screenshots/staplarna_splash.png" width="94">](staplarna/) | **[TZAAR](staplarna/)** | Capture and stack pieces across a hex board; lose a whole piece type and you lose the game. |

## Card & engine games — you vs the AI

| | Game | About |
|:-:|---|---|
| [<img src="expeditionen/screenshots/expeditionen_splash.png" width="94">](expeditionen/) | **[Lost Cities](expeditionen/)** | Commit cards to expeditions in ascending order — or cut your losses and discard. |
| [<img src="geishorna/screenshots/geishorna_splash.png" width="94">](geishorna/) | **[Hanamikoji](geishorna/)** | Spend four one-shot action markers to win the favour of more geishas than your rival. |
| [<img src="juvelerna/screenshots/juvelerna_splash.png" width="94">](juvelerna/) | **[Splendor](juvelerna/)** | Collect gems, buy cards that become permanent discounts, and race to 15 prestige. |
| [<img src="lapptacket/screenshots/lapptacket_splash.png" width="94">](lapptacket/) | **[Patchwork](lapptacket/)** | Buy polyomino patches and quilt your 9×9 board more efficiently than your opponent. |
| [<img src="mosaik/screenshots/mosaik_splash.png" width="94">](mosaik/) | **[Azul (Mosaik)](mosaik/)** | Draft coloured tiles from the factories and score them onto your wall. |
| [<img src="sushi/screenshots/sushi_splash.png" width="94">](sushi/) | **[Sushi Go!](sushi/)** | Draft a hand of cards passed back and forth for the best sushi combos. |

## Solo logic puzzles

| | Game | About |
|:-:|---|---|
| [<img src="2048/screenshots/2048_splash.png" width="94">](2048/) | **[2048](2048/)** | Swipe to slide and merge tiles until one reaches 2048. |
| [<img src="akari/screenshots/akari_splash.png" width="94">](akari/) | **[Akari (Light Up)](akari/)** | Place bulbs to light every white cell, with no two bulbs seeing each other. |
| [<img src="hashiwokakero/screenshots/hashiwokakero_splash.png" width="94">](hashiwokakero/) | **[Hashiwokakero](hashiwokakero/)** | Join the islands with one or two bridges each into a single connected network. |
| [<img src="kakuro/screenshots/kakuro_splash.png" width="94">](kakuro/) | **[Kakuro](kakuro/)** | Fill each run with 1–9 to hit its clue's sum, never repeating a digit. |
| [<img src="lightsout/screenshots/lightsout_splash.png" width="94">](lightsout/) | **[Lights Out](lightsout/)** | Tapping a light toggles it and its neighbours; turn the whole grid off. |
| [<img src="nonogram/screenshots/nonogram_splash.png" width="94">](nonogram/) | **[Nonogram](nonogram/)** | Use the row and column clues to paint the hidden picture. |
| [<img src="nurikabe/screenshots/nurikabe_splash.png" width="94">](nurikabe/) | **[Nurikabe](nurikabe/)** | Paint a single connected sea around numbered islands of the right sizes. |
| [<img src="slitherlink/screenshots/slitherlink_splash.png" width="94">](slitherlink/) | **[Slitherlink](slitherlink/)** | Draw one closed loop that obeys every numbered edge-count clue. |
| [<img src="sudoku/screenshots/sudoku_splash.png" width="94">](sudoku/) | **[Sudoku](sudoku/)** | Fill the grid so every row, column, and box contains 1–9. |

## Deduction & word games

| | Game | About |
|:-:|---|---|
| [<img src="anagram/screenshots/anagram_splash.png" width="94">](anagram/) | **[Ordskrav](anagram/)** | Unscramble the Swedish anagram. |
| [<img src="bagels/screenshots/bagels_splash.png" width="94">](bagels/) | **[Bagels](bagels/)** | Deduce the secret number from Pico / Fermi / Bagels hints. |
| [<img src="blackbox/screenshots/blackbox_splash.png" width="94">](blackbox/) | **[Black Box](blackbox/)** | Fire rays into the box and locate the hidden atoms from how the beams deflect. |
| [<img src="bullscows/screenshots/bullscows_splash.png" width="94">](bullscows/) | **[Bulls & Cows](bullscows/)** | Crack the secret code from bulls-and-cows feedback. |
| [<img src="einstein/screenshots/einstein_splash.png" width="94">](einstein/) | **[Einstein's Riddle](einstein/)** | Solve the classic zebra logic-grid puzzle. |
| [<img src="jotto/screenshots/jotto_splash.png" width="94">](jotto/) | **[Jotto](jotto/)** | Guess the hidden word from the count of matching letters. |
| [<img src="mastermind/screenshots/mastermind_splash.png" width="94">](mastermind/) | **[Mastermind](mastermind/)** | Break the colour code — or let the device crack yours in Knuth "device guesses" mode. |

## Programmed movement

| | Game | About |
|:-:|---|---|
| [<img src="roborally/screenshots/01_splash.png" width="94">](roborally/) | **[Robo Rally](roborally/)** | Program a sequence of movement cards to race your robot across a hazardous factory floor, solo vs AI robots. |

## Text adventures

| | Game | About |
|:-:|---|---|
| [<img src="grottan/screenshots/01_splash.png" width="94">](grottan/) | **[Grottan](grottan/)** | A tap-driven port of Colossal Cave Adventure — explore, take, and puzzle your way through the cave. |
| [<img src="studie/screenshots/01_splash.png" width="94">](studie/) | **[En Studie i Grått](studie/)** | A tap-driven detective mystery built on the same engine. |

## Utilities

| Folder | What it does |
|---|---|
| [boksynk](boksynk/) | **Boksynk** — syncs the Google Drive folder where you keep your books straight onto the reader over Wi-Fi: open the app, tap *Synka*, read. |
| [lasordning](lasordning/) | **Läsordning** — reads the device's library DB and shows your books grouped into series, in reading order. |
| [spelbutiken](spelbutiken/) | **Spelbutiken** — installs and updates all of the games *on the reader itself* from the latest GitHub release, over Wi-Fi. |
| [screentest](screentest/) | A screen-boundary diagnostic used while building these apps. |

---

## Installing on the reader — no computer needed

Once [Spelbutiken](spelbutiken/) is on the device (a one-time, computer-free
bootstrap: download `spelbutiken.install` from the
[latest release](https://github.com/jop3/ebook-games/releases/latest) with the
device browser, move + rename it to `applications/spelbutiken.app` with
KOReader's file browser), every game installs and updates from the reader
itself: open Spelbutiken → **Hämta listan** → **Installera allt**. Full
walkthrough in [spelbutiken/README.md](spelbutiken/README.md).

Releases are cut from the Actions tab (**Publish release** → run workflow with
a version tag); the workflow builds every game's ARM `.app` and attaches them
plus the Spelbutiken bootstrap to the release.

## Building

Each game is its own Go module with a cgo dependency on `libinkview`, so it's built
against the PocketBook Go SDK Docker image, not a plain `go build`:

```bash
docker run --rm -v "$PWD/<game>:/app" -w /app \
  sunsung/pocketbook-go-sdk:latest build -o <game>.app .
```

The [`Build .app binaries`](.github/workflows/build.yml) GitHub Actions workflow builds
every module's `.app` in the cloud on pushes to `main`, on tags, and on releases; the
[`CI (all games)`](.github/workflows/ci.yml) workflow runs every game's unit tests plus
the headless play-test suite on every push.

Compiled `.app`/`.exe` binaries and local debug/log output are gitignored; only source
(and the READMEs' screenshots) is tracked here.

## Playing headlessly

Every game runs under a pure-Go inkview emulator with no device or Docker, driving the
real touch UI:

```bash
playtest/play.sh <game>      # one game's play-test suite
playtest/play.sh all         # emulator self-tests + every game
```

The screenshots throughout these READMEs are captured by that same harness.

## Installing on the device

Copy the built `<game>.app` to `applications/` on the device's USB volume, eject, then
find it under **Apps → User Applications** on the reader.
