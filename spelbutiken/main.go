// Command spelbutiken ("the game shop") installs and updates the ebook-games
// library directly ON the PocketBook — no computer needed. It fetches the
// latest GitHub release of jop3/ebook-games over the device's own Wi-Fi, lists
// every game with its install state, and downloads tapped games straight into
// /mnt/ext1/applications/.
//
// Bootstrap (one time, also computer-free): download the `spelbutiken.install`
// asset with the device browser, then use KOReader's file browser to move it
// to applications/ and rename it to spelbutiken.app. After that, this app
// keeps everything (including itself) up to date. See README.md.
//
// Pure logic (GitHub API, install planning, zip unpack, manifest) lives in the
// ink-free ./store package so it is unit- and play-testable off-device.
package main

import (
	"context"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"spelbutiken/store"
)

// repo is the GitHub repository whose latest release is the game catalogue.
const repo = "jop3/ebook-games"

// appsDirDevice is where PocketBook looks for user applications.
const appsDirDevice = "/mnt/ext1/applications"

type app struct {
	fonts *Fonts

	client   *store.Client
	appsDir  string
	manifest map[string]string
	manPath  string

	release store.Release
	items   []store.Item
	status  string // one-line message under the title
	fetched bool

	top      int // first visible row
	rowRects []image.Rectangle
	buttons  []Button

	netReady bool
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App --------------------------------------------------------------------

// Init only fills fields a play test hasn't already injected, so tests can
// point the app at a temp dir and a fake release server before Boot.
func (a *app) Init() error {
	a.fonts = InitFonts()
	if a.appsDir == "" {
		a.appsDir = appsDirDevice
	}
	if a.manPath == "" {
		a.manPath = "spelbutiken_installed.json" // cwd = the app's own dir
	}
	if a.client == nil {
		a.client = &store.Client{
			API:  "https://api.github.com",
			Repo: repo,
			HTTP: &http.Client{Timeout: 5 * time.Minute},
		}
	}
	a.manifest = store.LoadManifest(a.manPath)
	a.status = "Tryck »Hämta listan« för att se spelen."
	ink.Repaint()
	return nil
}

func (a *app) Close() error {
	if a.fonts != nil {
		a.fonts.Close()
	}
	return nil
}

func (a *app) Draw() {
	screen := ink.ScreenSize()
	ink.ClearScreen()

	// Title + release line.
	a.fonts.Title.SetActive(ink.Black)
	ink.DrawString(image.Pt(24, topMargin+20), "Spelbutiken")
	a.fonts.Small.SetActive(ink.DarkGray)
	sub := repo
	if a.fetched {
		sub = "Utgåva " + a.release.Tag + " · " + itoa(len(a.items)) + " spel"
	}
	ink.DrawString(image.Pt(24, topMargin+76), ellipsize(sub, screen.X-48))
	ink.DrawLine(image.Pt(0, titleBarH), image.Pt(screen.X, titleBarH), ink.Black)

	areaTop := titleBarH + 10
	areaBottom := screen.Y - buttonBarH

	// Status line (wrapped — errors can be long).
	y := areaTop
	if a.status != "" {
		a.fonts.Small.SetActive(ink.Black)
		for _, ln := range wrapText(a.status, screen.X-48) {
			ink.DrawString(image.Pt(24, y), ln)
			y += 38
		}
		y += 10
	}

	// Game rows.
	a.rowRects = a.rowRects[:0]
	if a.fetched {
		visible := (areaBottom - y) / rowH
		if a.top < 0 {
			a.top = 0
		}
		if a.top > len(a.items)-1 {
			a.top = maxInt(0, len(a.items)-1)
		}
		end := minInt(len(a.items), a.top+visible)
		for i := a.top; i < end; i++ {
			r := image.Rect(0, y, screen.X, y+rowH)
			a.drawRow(r, a.items[i], screen.X)
			a.rowRects = append(a.rowRects, r)
			y += rowH
		}
	}

	a.buttons = drawButtonBar(screen, a.fonts,
		[]string{"▲", "▼", "Hämta listan", "Installera allt"},
		[]string{"up", "down", "fetch", "all"})
	ink.FullUpdate()
}

func (a *app) drawRow(r image.Rectangle, it store.Item, screenW int) {
	a.fonts.Head.SetActive(ink.Black)
	title := ellipsize(it.Name, screenW-48)
	ink.DrawString(image.Pt(r.Min.X+24, r.Min.Y+14), title)
	a.fonts.Small.SetActive(ink.DarkGray)
	var sub string
	switch it.State {
	case store.StateInstalled:
		sub = "installerad · " + a.release.Tag
	case store.StateUpdatable:
		sub = "uppdatering finns — tryck för att hämta"
	default:
		sub = "ny — tryck för att installera"
	}
	ink.DrawString(image.Pt(r.Min.X+48, r.Min.Y+62), ellipsize(sub, screenW-72))
	ink.DrawLine(image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Max.X, r.Max.Y), ink.LightGray)
}

// --- actions ----------------------------------------------------------------------

// connectNet brings the device network up once (on hardware this may pop the
// system Wi-Fi dialog) and pre-warms the TLS certificate pool.
func (a *app) connectNet() bool {
	if a.netReady {
		return true
	}
	if err := ink.ConnectDefault(); err != nil {
		a.status = "Ingen nätverksanslutning: " + err.Error()
		return false
	}
	_ = ink.InitCerts()
	a.netReady = true
	return true
}

// fetchList downloads the latest-release listing and rebuilds the rows. Drawn
// synchronously first so the progress text is visible during the fetch (a
// queued Repaint would only render after this blocking call returned).
func (a *app) fetchList() {
	a.status = "Hämtar listan …"
	a.Draw()
	if !a.connectNet() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rel, err := a.client.LatestRelease(ctx)
	if err != nil {
		a.status = "Kunde inte hämta listan: " + err.Error()
		return
	}
	a.release = rel
	a.replan()
	a.fetched = true
	a.status = ""
}

// replan recomputes every row's install state from disk + manifest.
func (a *app) replan() {
	a.items = store.Plan(a.release, a.appsDir, a.manifest)
}

// install downloads one game and marks it in the manifest.
func (a *app) install(it store.Item) {
	a.status = "Hämtar " + it.Name + " …"
	a.Draw()
	if !a.connectNet() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := a.client.Install(ctx, it.Asset, a.appsDir); err != nil {
		a.status = it.Name + ": " + err.Error()
		return
	}
	a.manifest[it.Name] = a.release.Tag
	_ = store.SaveManifest(a.manPath, a.manifest)
	a.replan()
	a.status = it.Name + " installerat. Nya spel syns under Program (starta ev. om läsplattan)."
}

// installAll fetches every game that is new or updatable.
func (a *app) installAll() {
	todo := 0
	for _, it := range a.items {
		if it.State != store.StateInstalled {
			todo++
		}
	}
	if todo == 0 {
		a.status = "Allt är redan installerat."
		return
	}
	done := 0
	for _, it := range a.items {
		if it.State == store.StateInstalled {
			continue
		}
		done++
		a.status = "Hämtar " + it.Name + " (" + itoa(done) + "/" + itoa(todo) + ") …"
		a.Draw()
		if !a.connectNet() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := a.client.Install(ctx, it.Asset, a.appsDir)
		cancel()
		if err != nil {
			a.status = it.Name + ": " + err.Error()
			return
		}
		a.manifest[it.Name] = a.release.Tag
		_ = store.SaveManifest(a.manPath, a.manifest)
	}
	a.replan()
	a.status = itoa(done) + " spel installerade. Nya spel syns under Program (starta ev. om läsplattan)."
}

// --- input ------------------------------------------------------------------------

func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) handleTap(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			switch b.ID {
			case "up":
				a.top -= 5
			case "down":
				a.top += 5
			case "fetch":
				a.fetchList()
			case "all":
				if a.fetched {
					a.installAll()
				} else {
					a.fetchList()
				}
			}
			ink.Repaint()
			return true
		}
	}
	for k, r := range a.rowRects {
		if p.In(r) {
			i := a.top + k
			if i >= 0 && i < len(a.items) {
				a.install(a.items[i])
				ink.Repaint()
				return true
			}
		}
	}
	return false
}

// Key: Back stays unhandled so the firmware exits the app (single screen).
func (a *app) Key(e ink.KeyEvent) bool            { return false }
func (a *app) Orientation(o ink.Orientation) bool { return false }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
