# Screentest (`screentest.app`)

A tiny diagnostic app that pinpoints the real drawable height of the PocketBook Verse Pro screen.

## About

**Screentest** is a developer tool, not a game. The Verse Pro's reported screen height (`1448`) is larger than the region the framebuffer actually shows without wrapping — content drawn too low wraps back to the top. This app finds the exact boundary so every other app in this repo can lay out bottom-up against a safe usable height (see `POCKETBOOK_GAMEDEV_GUIDE.md` §5).

It draws fine, labeled horizontal lines from `y=1300` to `y=1447`, one every 10 px. Whatever is still visible at the bottom of the screen is inside the real buffer; the first label value that has jumped to the top marks the wrap boundary.

## How to use

- Launch it from **Apps → User Applications** and read off the lowest label still at the bottom of the screen — that's the last safe y-coordinate.
- Use the result as the `usableH` constant when laying out a new app.

## Building

Built against the PocketBook Go SDK — see the repo [README](../README.md) and [POCKETBOOK_GAMEDEV_GUIDE.md](../POCKETBOOK_GAMEDEV_GUIDE.md).

```bash
docker run --rm -v "$PWD/screentest:/app" -w /app sunsung/pocketbook-go-sdk:latest build -o screentest.app .
```

Copy `screentest.app` into the device's `applications/` folder.
