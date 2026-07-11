# Spelbutiken (`spelbutiken.app`)

Install and update every game in this repo **directly on the PocketBook — no
computer needed**. Spelbutiken fetches the latest
[GitHub release](https://github.com/jop3/ebook-games/releases/latest) over the
reader's own Wi-Fi, shows each game as *ny* (new), *uppdatering finns*
(update available) or *installerad* (up to date), and downloads tapped games
straight into `applications/`.

## One-time setup (also computer-free)

You install Spelbutiken itself once, using only the reader. You need Wi-Fi and
KOReader (already on the device — it's the file manager for the last step).

1. **Download the bootstrap file.** Open the PocketBook **Browser** and go to

   `github.com/jop3/ebook-games/releases/latest`

   Scroll to the assets and download **`spelbutiken.install`**. It lands in
   the `Download` folder on the device.

2. **Move it into place with KOReader.** Open **KOReader** → its file browser.
   (If you don't see the file, enable showing all files: top menu → the
   gear/file-browser settings → *Show unsupported files*.) Navigate to
   `Download`, long-press `spelbutiken.install` → **Copy**. Navigate to
   `applications`, paste it there, then long-press it → **Rename** to

   `spelbutiken.app`

3. **Run it.** Exit KOReader. Open **Apps / Program** (restart the reader if
   the new entry doesn't show yet) and start **spelbutiken**. Tap
   **Hämta listan**, then **Installera allt** — or tap individual games.
   Newly installed games appear in the same Apps list.

Spelbutiken updates itself the same way it updates games — it is one of the
rows in its own list.

## How it works

- The list comes from the repo's latest GitHub release (`releases/latest` via
  the GitHub API); every `<game>.zip` asset is one row.
- A game already on the device but installed by hand (USB, unknown version) is
  offered as *uppdatering finns* — downloading it again is always safe.
- Downloads are unpacked in memory and written atomically (`.tmp` + rename),
  so a dropped Wi-Fi connection can never leave a half-written game behind.
- What was installed from which release is recorded in
  `spelbutiken_installed.json` next to the binary.
- Games installed this way appear under **Apps / Program** automatically. The
  fancy per-game icons on the main screen's Games panel still come from the
  `view.json` recipe (guide §8), which needs a USB session — cosmetic only.

The pure logic (GitHub API client, install planning, zip unpacking, manifest)
lives in the SDK-free `store/` package and is unit-tested; the play-test suite
drives the whole install flow through the real touch UI against a local fake
release server (`playtest/play.sh spelbutiken`).

## Building

Built against the PocketBook Go SDK like every other module — see the repo
[README](../README.md) and [POCKETBOOK_GAMEDEV_GUIDE.md](../POCKETBOOK_GAMEDEV_GUIDE.md).

```bash
docker run --rm -v "$PWD/spelbutiken:/app" -w /app sunsung/pocketbook-go-sdk:latest build -o spelbutiken.app .
```

The release workflow (`.github/workflows/release.yml`) attaches both the
per-game zips and the raw `spelbutiken.install` bootstrap to every release.
