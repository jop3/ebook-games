// Command boksynk syncs the user's Google Drive book folder straight onto the
// PocketBook — no computer needed. Turn on Wi-Fi, open the app, tap »Synka«:
// every epub/pdf/… in the shared Drive folder that is new or changed lands in
// /mnt/ext1/Books/Drive, subfolders preserved, and the library picks them up.
//
// One-time setup (see README.md): share the Drive folder as "anyone with the
// link", create a free Google API key, and paste both into the boksynk.json
// file this app writes next to itself on first launch.
//
// Pure logic (Drive API, sync planning, atomic downloads, manifest) lives in
// the ink-free ./drive package so it is unit- and play-testable off-device.
package main

import (
	"context"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	ink "github.com/dennwc/inkview"

	"boksynk/drive"
)

// driveAPI is the real Google Drive v3 endpoint (tests point client.API at a
// local fake instead).
const driveAPI = "https://www.googleapis.com/drive/v3"

const (
	cfgFile = "boksynk.json"        // user setup, next to the binary
	manFile = "boksynk_synced.json" // relpath -> md5 of what we downloaded
)

// setupHelp is shown (wrapped) whenever the app is not configured yet.
const setupHelp = "Inte inställd ännu. Anslut läsplattan med USB och öppna filen " +
	"applications/boksynk.json på en dator. Klistra in (1) delningslänken till din " +
	"Drive-mapp (delad som »alla med länken«) och (2) en Google API-nyckel med Drive API " +
	"aktiverat. Se README för stegen. Starta sedan om appen."

type screen int

const (
	screenSplash screen = iota // shown on launch; tap → main
	screenMain
)

type app struct {
	fonts *Fonts
	scr   screen

	cfgPath string
	manPath string
	cfg     drive.Config
	cfgErr  string

	client   *drive.Client
	manifest map[string]string

	files   []drive.RemoteFile
	items   []drive.Item
	fetched bool
	status  string // one-line (wrapped) message under the title

	top      int // first visible row
	rowRects []image.Rectangle
	buttons  []Button

	netReady bool
	downY    int // pointer-down y, for swipe scrolling (§5a)
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
// point the app at a temp dir and a fake Drive server before Boot.
func (a *app) Init() error {
	a.fonts = InitFonts()
	if a.cfgPath == "" {
		a.cfgPath = cfgFile // cwd = the app's own dir
	}
	if a.manPath == "" {
		a.manPath = manFile
	}
	if a.client == nil {
		a.client = &drive.Client{
			API:  driveAPI,
			HTTP: &http.Client{Timeout: 5 * time.Minute},
		}
	}
	a.manifest = drive.LoadManifest(a.manPath)
	a.loadConfig()
	if a.cfgErr != "" {
		a.status = a.cfgErr
	} else if !a.cfg.Ready() {
		a.status = setupHelp
	} else {
		a.status = "Tryck »Synka« för att hämta det senaste från Drive."
	}
	ink.Repaint()
	return nil
}

// loadConfig (re)reads boksynk.json, writes the fill-in template on first run,
// and hands the API key to the client. Re-run on every fetch so edits made
// with an on-device editor are picked up without restarting.
func (a *app) loadConfig() {
	cfg, err := drive.LoadConfig(a.cfgPath)
	if err != nil {
		a.cfgErr = "Fel i " + filepath.Base(a.cfgPath) + ": " + err.Error()
		return
	}
	a.cfgErr = ""
	a.cfg = cfg
	if !cfg.Ready() {
		_ = drive.WriteTemplate(a.cfgPath)
		return
	}
	a.client.Key = cfg.APIKey
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

	if a.scr == screenSplash {
		drawSplash(screen, a.fonts)
		ink.FullUpdate()
		return
	}

	// Title + subtitle.
	a.fonts.Title.SetActive(ink.Black)
	ink.DrawString(image.Pt(24, topMargin+20), "Boksynk")
	a.fonts.Small.SetActive(ink.DarkGray)
	sub := "Google Drive → " + a.displayDir()
	if a.fetched {
		n := a.pendingCount()
		sub = itoa(len(a.items)) + " böcker på Drive · "
		if n == 0 {
			sub += "allt synkat"
		} else {
			sub += itoa(n) + " att hämta"
		}
	}
	ink.DrawString(image.Pt(24, topMargin+76), ellipsize(sub, screen.X-48))
	ink.DrawLine(image.Pt(0, titleBarH), image.Pt(screen.X, titleBarH), ink.Black)

	areaTop := titleBarH + 10
	areaBottom := usableH - buttonBarH

	// Status line (wrapped — setup help and errors are long).
	y := areaTop
	if a.status != "" {
		a.fonts.Small.SetActive(ink.Black)
		for _, ln := range wrapText(a.status, screen.X-48) {
			ink.DrawString(image.Pt(24, y), ln)
			y += 38
		}
		y += 10
	}

	// Book rows.
	a.rowRects = a.rowRects[:0]
	if a.fetched {
		visible := (areaBottom - y) / rowH
		if a.top > len(a.items)-visible {
			a.top = len(a.items) - visible
		}
		if a.top < 0 {
			a.top = 0
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
		[]string{"▲", "▼", "Uppdatera", "Synka"},
		[]string{"up", "down", "fetch", "sync"})
	ink.FullUpdate()
}

func (a *app) drawRow(r image.Rectangle, it drive.Item, screenW int) {
	a.fonts.Head.SetActive(ink.Black)
	ink.DrawString(image.Pt(r.Min.X+24, r.Min.Y+14), ellipsize(it.File.Name, screenW-48))
	a.fonts.Small.SetActive(ink.DarkGray)
	sub := fmtSize(it.File.Size)
	if dir := filepath.Dir(it.File.RelPath); dir != "." {
		sub += " · " + dir
	}
	switch it.State {
	case drive.StateNew:
		sub += " · ny — hämtas vid synk"
	case drive.StateChanged:
		sub += " · ändrad på Drive — hämtas vid synk"
	default:
		sub += " · synkad"
	}
	ink.DrawString(image.Pt(r.Min.X+48, r.Min.Y+62), ellipsize(sub, screenW-72))
	ink.DrawLine(image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Max.X, r.Max.Y), ink.LightGray)
}

// displayDir shortens the books dir for the subtitle ("Books/Drive").
func (a *app) displayDir() string {
	return strings.TrimPrefix(a.cfg.Dir(), "/mnt/ext1/")
}

func (a *app) pendingCount() int {
	n := 0
	for _, it := range a.items {
		if it.State != drive.StateSynced {
			n++
		}
	}
	return n
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

// fetchList lists the Drive folder and rebuilds the rows. Drawn synchronously
// first so the progress text is visible during the fetch (a queued Repaint
// would only render after this blocking call returned).
func (a *app) fetchList() {
	a.loadConfig()
	if a.cfgErr != "" {
		a.status = a.cfgErr
		return
	}
	if !a.cfg.Ready() {
		a.status = setupHelp
		return
	}
	a.status = "Hämtar boklistan …"
	a.Draw()
	if !a.connectNet() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	files, err := a.client.ListFolder(ctx, a.cfg)
	if err != nil {
		a.status = "Kunde inte läsa Drive-mappen: " + err.Error()
		return
	}
	a.files = files
	a.replan()
	a.fetched = true
	if len(files) == 0 {
		a.status = "Mappen innehåller inga böcker (kontrollera delningslänken)."
	} else {
		a.status = ""
	}
}

// replan recomputes every row's sync state from disk + manifest.
func (a *app) replan() {
	a.items = drive.Plan(a.files, a.cfg.Dir(), a.manifest)
}

// download fetches one book and records it in the manifest.
func (a *app) download(it drive.Item) bool {
	a.status = "Hämtar " + it.File.Name + " …"
	a.Draw()
	if !a.connectNet() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	sum, err := a.client.Download(ctx, it.File, a.cfg.Dir())
	if err != nil {
		a.status = it.File.Name + ": " + err.Error()
		return false
	}
	a.manifest[it.File.RelPath] = sum
	_ = drive.SaveManifest(a.manPath, a.manifest)
	return true
}

// downloadOne is a tap on a single row.
func (a *app) downloadOne(it drive.Item) {
	if it.State == drive.StateSynced {
		a.status = it.File.Name + " är redan synkad."
		return
	}
	if !a.download(it) {
		return
	}
	a.replan()
	a.status = it.File.Name + " hämtad till " + a.displayDir() + "."
}

// syncAll fetches every book that is new or changed. This is THE button: from
// a cold start it lists the folder first, then downloads what's missing.
func (a *app) syncAll() {
	if !a.fetched {
		a.fetchList()
		if !a.fetched {
			return
		}
	}
	todo := 0
	for _, it := range a.items {
		if it.State != drive.StateSynced {
			todo++
		}
	}
	if todo == 0 {
		a.status = "Allt är redan synkat."
		return
	}
	done := 0
	for _, it := range a.items {
		if it.State == drive.StateSynced {
			continue
		}
		done++
		a.status = "Hämtar " + it.File.Name + " (" + itoa(done) + "/" + itoa(todo) + ") …"
		a.Draw()
		if !a.connectNet() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		sum, err := a.client.Download(ctx, it.File, a.cfg.Dir())
		cancel()
		if err != nil {
			a.status = it.File.Name + ": " + err.Error()
			return
		}
		a.manifest[it.File.RelPath] = sum
		_ = drive.SaveManifest(a.manPath, a.manifest)
	}
	a.replan()
	word := "böcker hämtade"
	if done == 1 {
		word = "bok hämtad"
	}
	a.status = "Klart! " + itoa(done) + " " + word + " till " + a.displayDir() +
		". Nya böcker syns i biblioteket efter en sökning."
}

// --- input ------------------------------------------------------------------------

func (a *app) Pointer(e ink.PointerEvent) bool {
	switch e.State {
	case ink.PointerDown:
		a.downY = e.Point.Y
		return true
	case ink.PointerUp:
		return a.handleUp(e.Point)
	}
	return false
}

func (a *app) Touch(e ink.TouchEvent) bool {
	switch e.State {
	case ink.TouchDown:
		a.downY = e.Point.Y
		return true
	case ink.TouchUp:
		return a.handleUp(e.Point)
	}
	return false
}

// handleUp routes a finger lift: a long vertical travel is a scroll gesture,
// anything else is a tap (§5a).
func (a *app) handleUp(p image.Point) bool {
	dy := p.Y - a.downY
	if a.scr == screenMain && a.fetched && (dy >= swipeMin || dy <= -swipeMin) {
		a.top -= dy / rowH // swipe up (dy<0) → later rows
		ink.Repaint()
		return true
	}
	return a.handleTap(p)
}

func (a *app) handleTap(p image.Point) bool {
	if a.scr == screenSplash {
		a.scr = screenMain
		ink.Repaint()
		return true
	}
	for _, b := range a.buttons {
		if b.Hit(p) {
			switch b.ID {
			case "up":
				a.top -= 5
			case "down":
				a.top += 5
			case "fetch":
				a.fetchList()
			case "sync":
				a.syncAll()
			}
			ink.Repaint()
			return true
		}
	}
	for k, r := range a.rowRects {
		if p.In(r) {
			i := a.top + k
			if i >= 0 && i < len(a.items) {
				a.downloadOne(a.items[i])
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
