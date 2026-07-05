# ebook-games

Games and small apps built for the **PocketBook Verse Pro (PB634)** e-ink reader —
1072×1448 portrait, greyscale, 32-bit ARM. Built with the Go SDK
[dennwc/inkview](https://github.com/dennwc/inkview) (vendored per-project under
`third_party/inkview`).

See [POCKETBOOK_GAMEDEV_GUIDE.md](POCKETBOOK_GAMEDEV_GUIDE.md) for the full toolchain
writeup (WSL2 + Docker build, headless emulator, deploy, and every trap hit along the way)
before starting a new game.

## Games

| Folder | Game |
|---|---|
| [irad](irad/) | I rad — X-in-a-row (tre/fyra/fem i rad, Gomoku), 1–4 players |
| [mastermind](mastermind/) | Mastermind, incl. a "device guesses" (Knuth) mode |
| [blackbox](blackbox/) | Black Box |
| [einstein](einstein/) | Einsteins Gåta (Einstein's Riddle) |
| [othello](othello/) | Othello / Reversi |
| [nonogram](nonogram/) | Nonogram (picross) |
| [hex](hex/) | Hex |
| [bullscows](bullscows/) | Bulls & Cows |
| [sudoku](sudoku/) | Sudoku |
| [lightsout](lightsout/) | Lights Out |
| [nim](nim/) | Nim (with a perfect-play AI) |
| [anagram](anagram/) | Ordskrav (Swedish anagram game) |
| [bagels](bagels/) | Bagels |
| [jotto](jotto/) | Jotto (Swedish word game) |
| [akari](akari/) | Akari (light-up puzzle) |
| [slitherlink](slitherlink/) | Slitherlink |
| [quarto](quarto/) | Quarto |
| [hashiwokakero](hashiwokakero/) | Hashiwokakero (bridges) |
| [kakuro](kakuro/) | Kakuro |
| [nurikabe](nurikabe/) | Nurikabe |
| [roborally](roborally/) | Robo Rally (simplified — 1 human vs 1–3 AI robots; program registers, race the checkpoints) |
| [2048](2048/) | 2048 — sliding-tile swipe puzzle on a 4×4 grid, single player |
| [goban](goban/) | Go (baduk/weiqi) — 9×9/13×13/19×19, area scoring, ko, mark-dead phase, 9×9 AI |
| [hasami](hasami/) | Hasami shogi — custodial + corner capture; hot-seat or minimax AI |
| [shong](shong/) | Shong — a 4×6 chess-like duel with a genuinely strong AI |
| [munkar](munkar/) | Munkar — line-glyph placement with inverted capture (based on Donuts) |
| [sushi](sushi/) | Sushi — card-drafting, 1 human vs 1–4 AI (based on Sushi Go!) |
| [mosaik](mosaik/) | Mosaik — tile-drafting + wall-scoring, 2 players (based on Azul) |
| [ringar](ringar/) | Ringar — ring-and-marker game on an 85-point hex board (based on YINSH) |
| [hexa](hexa/) | Hexa — hex-tile mosaic, line up 6 of your own tiles (based on Six) |
| [grottan](grottan/) | Grottan — tap-driven text adventure (a port of Colossal Cave) |
| [studie](studie/) | En Studie i Grått — tap-driven locked-room mystery on the Grottan engine |
| [screentest](screentest/) | Diagnostic app for screen/input debugging |

## Other apps

| Folder | What it does |
|---|---|
| [lasordning](lasordning/) | Reads the device's library DB and shows books grouped by series in reading order |

## Building

Each game is its own Go module with a cgo dependency on `libinkview`, so it's built
against the PocketBook Go SDK Docker image, not a plain `go build`:

```bash
docker run --rm -v "$PWD/<game>:/app" -w /app \
  sunsung/pocketbook-go-sdk:latest build -o <game>.app .
```

Some games (e.g. `irad`) have a GitHub Actions workflow that builds the `.app` in the
cloud on push — see that game's `.github/workflows/`.

Compiled `.app`/`.exe` binaries and local debug/log output are gitignored; only source
is tracked here.

## Installing on the device

Copy the built `<game>.app` to `applications/` on the device's USB volume, eject, then
find it under **Apps → User Applications** on the reader.