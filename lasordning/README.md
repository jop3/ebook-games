# Läsordning (`lasordning.app`)

A PocketBook Verse Pro utility that reads your book library and tells you which books belong to a series and in what order to read them.

## About

**Läsordning** ("reading order") is not a game — it's a companion app for the reader itself. It scans the device's book database, groups your books into series, works out each book's position in its series, and presents them so you always know where to start and how to continue.

Series information is resolved through a fallback chain: first the library database's own series columns, then a heuristic parse of the book title, then online lookups (Wikidata, then the Wikipedia infobox). You can correct any book's number by hand, and the fix is written back to the device library.

The pure logic (data model, series detection, grouping) lives in the SDK-free `series/` package and is unit-tested; `main.go`, `ui.go`, and the `screen_*.go` files handle database access, networking, rendering, and input.

## How to use

- Launch it from **Apps → User Applications**. It reads the library DB directly — no setup.
- The list screen groups books by detected series, in reading order, and flags anything ambiguous.
- Tap a series to see its detail view; tap a book to edit its series number if the automatic detection got it wrong. Edits are saved back to the library.
- Online series lookups require the device to have network access; without it, detection falls back to the DB columns and title heuristics.

## Building

Built against the PocketBook Go SDK — see the repo [README](../README.md) and [POCKETBOOK_GAMEDEV_GUIDE.md](../POCKETBOOK_GAMEDEV_GUIDE.md).

```bash
docker run --rm -v "$PWD/lasordning:/app" -w /app sunsung/pocketbook-go-sdk:latest build -o lasordning.app .
```

Copy `lasordning.app` into the device's `applications/` folder. Unit tests for the series logic run with `go test ./series/` from the module directory.
