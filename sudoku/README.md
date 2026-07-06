# Sudoku (`sudoku.app`)

The classic number-placement puzzle, with pencil marks and three difficulties, tuned for e-ink.

<p align="center"><img src="screenshots/sudoku_splash.png" width="300" alt="Sudoku splash"></p>

## About

Sudoku for the PocketBook Verse Pro, built on the `dennwc/inkview` SDK. Every puzzle is generated to have a unique solution, in three difficulties (Latt / Medel / Svar — Easy / Medium / Hard). The pure puzzle logic (generation, solving, conflict detection) lives in a separate, unit-tested `game` package. Given cells are drawn underlined; a pencil-note mode lets you jot candidate digits before committing.

## How to play

- **Goal:** fill the whole grid so each row, each column and each 3x3 box contains the digits 1–9 exactly once.
- Some cells are already filled (underlined) — these are fixed clues and cannot be changed.
- **Controls:** tap a cell to select it, then tap a digit 1–9 to fill it. Tap the same digit again to remove it.
- **Anteckn** (Notes) mode: toggle small pencil marks. Digits then fill the cell as little notes instead of a big value — handy for tracking possible options.
- **Sudda** (Erase) removes the digit or notes in the selected cell.
- **Klar?** (Check) inspects the board: conflicts are marked, and once everything is filled it tells you whether the solution is right.
- **Ny** (New) returns to the menu to pick a difficulty: Latt, Medel or Svar.

## Screenshots

<table>
  <tr>
    <td align="center"><img src="screenshots/sudoku_board.png" width="240"><br><sub>A puzzle in progress (player entries underlined)</sub></td>
    <td align="center"><img src="screenshots/sudoku_win.png" width="240"><br><sub>Solved and confirmed with "Klar?"</sub></td>
    <td align="center"><img src="screenshots/sudoku_rules.png" width="240"><br><sub>In-app Swedish rules</sub></td>
  </tr>
</table>

## Building

Built against the PocketBook Go SDK — see the repo [README](../README.md) and [POCKETBOOK_GAMEDEV_GUIDE.md](../POCKETBOOK_GAMEDEV_GUIDE.md).

```bash
docker run --rm -v "$PWD/sudoku:/app" -w /app sunsung/pocketbook-go-sdk:latest build -o sudoku.app .
```

Copy `sudoku.app` into the device's `applications/` folder. Headless tests: `playtest/play.sh sudoku`.
