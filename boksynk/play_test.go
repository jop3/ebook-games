//go:build playtest

package main

// Headless play tests for Boksynk under the pure-Go inkview emulator
// (playtest/play.sh boksynk). A local httptest server stands in for the
// Google Drive API, and the app is pointed at a temp config + books dir, so
// the whole sync flow — read config, list the shared folder, plan, download,
// atomic write, manifest update, row state change — runs through the real
// touch UI with no network.

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	ink "github.com/dennwc/inkview"

	"boksynk/drive"
)

const rootID = "root1234567890"

// fakeDrive is a minimal mutable Drive v3: files.list (q by parent, one file
// per page to exercise pagination) + files/{id}?alt=media.
type fakeDrive struct {
	mu      sync.Mutex
	parents map[string]string // id -> parent id ("" = not listed)
	names   map[string]string // id -> name
	folders map[string]bool   // id -> is folder
	bodies  map[string][]byte // id -> content (files only)
	key     string            // required API key
}

func (d *fakeDrive) setFile(id, parent, name string, body []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.parents[id], d.names[id], d.bodies[id] = parent, name, body
}

func (d *fakeDrive) setFolder(id, parent, name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.parents[id], d.names[id], d.folders[id] = parent, name, true
}

func md5hex(b []byte) string {
	h := md5.Sum(b)
	return hex.EncodeToString(h[:])
}

func (d *fakeDrive) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		if r.URL.Query().Get("key") != d.key {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "API key not valid"}})
			return
		}
		q := r.URL.Query().Get("q")
		parent := ""
		if i := strings.Index(q, "'"); i >= 0 {
			if j := strings.Index(q[i+1:], "'"); j >= 0 {
				parent = q[i+1 : i+1+j]
			}
		}
		type jf struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			MimeType string `json:"mimeType"`
			Size     string `json:"size,omitempty"`
			MD5      string `json:"md5Checksum,omitempty"`
		}
		var in []jf
		for id, p := range d.parents {
			if p != parent {
				continue
			}
			e := jf{ID: id, Name: d.names[id], MimeType: "application/octet-stream"}
			if d.folders[id] {
				e.MimeType = "application/vnd.google-apps.folder"
			} else {
				e.Size = strconv.Itoa(len(d.bodies[id]))
				e.MD5 = md5hex(d.bodies[id])
			}
			in = append(in, e)
		}
		// Deterministic paging order.
		for i := 0; i < len(in); i++ {
			for j := i + 1; j < len(in); j++ {
				if in[j].ID < in[i].ID {
					in[i], in[j] = in[j], in[i]
				}
			}
		}
		start := 0
		if pt := r.URL.Query().Get("pageToken"); pt != "" {
			start, _ = strconv.Atoi(pt)
		}
		out := struct {
			NextPageToken string `json:"nextPageToken,omitempty"`
			Files         []jf   `json:"files"`
		}{}
		if start < len(in) {
			out.Files = in[start : start+1]
			if start+1 < len(in) {
				out.NextPageToken = strconv.Itoa(start + 1)
			}
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		id := strings.TrimPrefix(r.URL.Path, "/files/")
		if b, ok := d.bodies[id]; ok && r.URL.Query().Get("alt") == "media" {
			w.Write(b)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "File not found"}})
	})
	return mux
}

// newLibrary is the standard test share: two books at the top level, one book
// in a subfolder, one non-book that must never appear.
func newLibrary(key string) *fakeDrive {
	d := &fakeDrive{
		parents: map[string]string{}, names: map[string]string{},
		folders: map[string]bool{}, bodies: map[string][]byte{}, key: key,
	}
	d.setFolder(rootID, "", "Böcker")
	d.setFile("f1", rootID, "Dune.epub", []byte("dune-bytes"))
	d.setFile("f2", rootID, "Hyperion.epub", []byte("hyperion-bytes"))
	d.setFile("f3", rootID, "cover.jpg", []byte("not-a-book"))
	d.setFolder("sub1", rootID, "Serier")
	d.setFile("f4", "sub1", "Foundation.epub", []byte("foundation-bytes"))
	return d
}

// bootSync boots the app against a fake Drive and a temp device. If cfg is
// nil, no config file exists yet (first launch).
func bootSync(t *testing.T, d *fakeDrive, cfg *drive.Config) (*ink.Harness, *app, string) {
	t.Helper()
	srv := httptest.NewServer(d.handler())
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	books := filepath.Join(dir, "Books", "Drive")
	cfgPath := filepath.Join(dir, "boksynk.json")
	if cfg != nil {
		if cfg.BooksDir == "" {
			cfg.BooksDir = books
		}
		b, _ := json.Marshal(cfg)
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := &app{
		cfgPath: cfgPath,
		manPath: filepath.Join(dir, "boksynk_synced.json"),
		client:  &drive.Client{API: srv.URL, HTTP: srv.Client()},
	}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	return h, a, books
}

func enterMain(t *testing.T, h *ink.Harness) {
	t.Helper()
	if _, ok := h.FindText("Boksynk"); !ok {
		t.Fatal("splash title should be visible on launch")
	}
	h.TapXY(500, 700) // any tap dismisses the splash
}

// --- first launch: splash + unconfigured help -------------------------------------

func TestPlayBoksynkFirstLaunch(t *testing.T) {
	h, a, _ := bootSync(t, newLibrary("k"), nil)
	enterMain(t, h)

	if _, ok := h.FindTextContains("Inte inställd"); !ok {
		t.Fatal("an unconfigured app must show the setup help")
	}
	// First launch writes the fill-in template.
	b, err := os.ReadFile(a.cfgPath)
	if err != nil || !strings.Contains(string(b), "apiKey") {
		t.Fatalf("first launch should write a config template: %v %s", err, b)
	}
	// Tapping Synka while unconfigured keeps showing help, never crashes.
	if err := h.TapText("Synka"); err != nil {
		t.Fatal(err)
	}
	if a.fetched {
		t.Fatal("nothing must be fetched without config")
	}
	if _, ok := h.FindTextContains("Inte inställd"); !ok {
		t.Fatal("setup help should still be shown")
	}
}

// --- the headline flow: one tap syncs everything ----------------------------------

func TestPlayBoksynkSyncAllColdStart(t *testing.T) {
	h, a, books := bootSync(t, newLibrary("k"), &drive.Config{Folder: rootID, APIKey: "k"})
	enterMain(t, h)

	if err := h.TapText("Synka"); err != nil {
		t.Fatal(err)
	}

	// All three books (and ONLY books) are on the device, subfolders kept.
	for path, want := range map[string]string{
		"Dune.epub":              "dune-bytes",
		"Hyperion.epub":          "hyperion-bytes",
		"Serier/Foundation.epub": "foundation-bytes",
	} {
		b, err := os.ReadFile(filepath.Join(books, filepath.FromSlash(path)))
		if err != nil || string(b) != want {
			t.Fatalf("%s should be synced with content: %v %q", path, err, b)
		}
	}
	if _, err := os.Stat(filepath.Join(books, "cover.jpg")); !os.IsNotExist(err) {
		t.Fatal("non-book files must not be downloaded")
	}
	if got := drive.LoadManifest(a.manPath); len(got) != 3 {
		t.Fatalf("manifest should record 3 books, got %v", got)
	}
	if _, ok := h.FindTextContains("Klart! 3 böcker"); !ok {
		t.Fatal("the completion message should be shown")
	}
	if _, ok := h.FindTextContains("allt synkat"); !ok {
		t.Fatal("the subtitle should show everything is synced")
	}
	for _, it := range a.items {
		if it.State != drive.StateSynced {
			t.Fatalf("%s should be synced, got %v", it.File.RelPath, it.State)
		}
	}

	// A second sync is a no-op.
	if err := h.TapText("Synka"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.FindTextContains("redan synkat"); !ok {
		t.Fatal("re-syncing with nothing to do should say so")
	}
}

// --- list first, then fetch one book via its row -----------------------------------

func TestPlayBoksynkFetchAndRowTap(t *testing.T) {
	h, a, books := bootSync(t, newLibrary("k"), &drive.Config{Folder: rootID, APIKey: "k"})
	enterMain(t, h)

	if err := h.TapText("Uppdatera"); err != nil {
		t.Fatal(err)
	}
	if !a.fetched || len(a.items) != 3 {
		t.Fatalf("expected 3 book rows after refresh, got %v", a.items)
	}
	if _, ok := h.FindText("Dune.epub"); !ok {
		t.Fatal("book rows should be listed")
	}
	if _, ok := h.FindTextContains("3 att hämta"); !ok {
		t.Fatal("the subtitle should count pending books")
	}

	// Tap the first row (sorted by RelPath: Dune.epub first).
	if !h.TapRect(a.rowRects[0]) {
		t.Fatal("tapping a book row should be handled")
	}
	if b, err := os.ReadFile(filepath.Join(books, "Dune.epub")); err != nil || string(b) != "dune-bytes" {
		t.Fatalf("row tap should download that book: %v %q", err, b)
	}
	if a.items[0].State != drive.StateSynced {
		t.Fatalf("the tapped row should now be synced, got %v", a.items[0].State)
	}
	if a.pendingCount() != 2 {
		t.Fatalf("the other 2 books stay pending, got %d", a.pendingCount())
	}

	// Tapping an already-synced row does not re-download; it reports instead.
	if !h.TapRect(a.rowRects[0]) {
		t.Fatal("tapping a synced row should still be handled")
	}
	if _, ok := h.FindTextContains("redan synkad"); !ok {
		t.Fatal("tapping a synced row should say it is already synced")
	}
}

// --- a book edited on Drive is re-downloaded ---------------------------------------

func TestPlayBoksynkChangedBook(t *testing.T) {
	lib := newLibrary("k")
	h, a, books := bootSync(t, lib, &drive.Config{Folder: rootID, APIKey: "k"})
	enterMain(t, h)

	if err := h.TapText("Synka"); err != nil {
		t.Fatal(err)
	}

	// The book gets a new revision on Drive.
	lib.setFile("f1", rootID, "Dune.epub", []byte("dune-second-edition"))

	if err := h.TapText("Uppdatera"); err != nil {
		t.Fatal(err)
	}
	if a.items[0].State != drive.StateChanged {
		t.Fatalf("an edited book should be Changed, got %v", a.items[0].State)
	}
	if _, ok := h.FindTextContains("ändrad på Drive"); !ok {
		t.Fatal("the changed state should be visible in the row")
	}
	if err := h.TapText("Synka"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(books, "Dune.epub"))
	if err != nil || string(b) != "dune-second-edition" {
		t.Fatalf("the new revision should replace the old one: %v %q", err, b)
	}
	if _, ok := h.FindTextContains("Klart! 1 bok hämtad"); !ok {
		t.Fatal("only the changed book should have been fetched")
	}
}

// --- books already on the device (USB copy) are recognized -------------------------

func TestPlayBoksynkUSBCopyRecognized(t *testing.T) {
	h, a, books := bootSync(t, newLibrary("k"), &drive.Config{Folder: rootID, APIKey: "k"})
	// An identical Dune.epub was copied over USB long ago (no manifest entry).
	if err := os.MkdirAll(books, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(books, "Dune.epub"), []byte("dune-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	enterMain(t, h)
	if err := h.TapText("Uppdatera"); err != nil {
		t.Fatal(err)
	}
	if a.items[0].State != drive.StateSynced {
		t.Fatalf("an identical USB-copied book must count as synced, got %v", a.items[0].State)
	}
	if a.pendingCount() != 2 {
		t.Fatalf("only the 2 missing books should be pending, got %d", a.pendingCount())
	}
}

// --- failures fail soft --------------------------------------------------------------

func TestPlayBoksynkBadKey(t *testing.T) {
	h, a, _ := bootSync(t, newLibrary("good"), &drive.Config{Folder: rootID, APIKey: "wrong"})
	enterMain(t, h)
	if err := h.TapText("Synka"); err != nil {
		t.Fatal(err)
	}
	if a.fetched {
		t.Fatal("a failed listing must not mark the app as fetched")
	}
	if _, ok := h.FindTextContains("API-nyckeln"); !ok {
		t.Fatal("the error should point at the API key")
	}
	// The app stays alive and re-tappable.
	if err := h.TapText("Uppdatera"); err != nil {
		t.Fatal(err)
	}
}

// --- scrolling: buttons and swipe ---------------------------------------------------

func TestPlayBoksynkScroll(t *testing.T) {
	lib := newLibrary("k")
	for i := 0; i < 30; i++ {
		id := "x" + strconv.Itoa(100+i)
		lib.setFile(id, rootID, "Bok "+strconv.Itoa(100+i)+".epub", []byte("b"+strconv.Itoa(i)))
	}
	h, a, _ := bootSync(t, lib, &drive.Config{Folder: rootID, APIKey: "k"})
	enterMain(t, h)
	if err := h.TapText("Uppdatera"); err != nil {
		t.Fatal(err)
	}
	if len(a.items) != 33 {
		t.Fatalf("expected 33 rows, got %d", len(a.items))
	}
	if a.top != 0 {
		t.Fatal("listing starts at the top")
	}
	if err := h.TapText("▼"); err != nil {
		t.Fatal(err)
	}
	if a.top != 5 {
		t.Fatalf("▼ should scroll 5 rows, top=%d", a.top)
	}
	if err := h.TapText("▲"); err != nil {
		t.Fatal(err)
	}
	if a.top != 0 {
		t.Fatalf("▲ should scroll back, top=%d", a.top)
	}

	// Swipe up (finger travels from y=900 to y=400) scrolls forward…
	a.Pointer(ink.PointerEvent{Point: image.Pt(500, 900), State: ink.PointerDown})
	a.Pointer(ink.PointerEvent{Point: image.Pt(500, 400), State: ink.PointerUp})
	if a.top <= 0 {
		t.Fatalf("swipe up should scroll down the list, top=%d", a.top)
	}
	// …and a small travel is a tap, not a scroll.
	before := a.top
	a.Pointer(ink.PointerEvent{Point: image.Pt(10, 900), State: ink.PointerDown})
	a.Pointer(ink.PointerEvent{Point: image.Pt(10, 920), State: ink.PointerUp})
	if a.top != before {
		t.Fatal("a 20px travel must not scroll")
	}
}

// --- Screenshots of every screen for visual review ---------------------------

func TestPlayBoksynkScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, _, _ := bootSync(t, newLibrary("k"), &drive.Config{Folder: rootID, APIKey: "k"})
	if err := h.Screenshot(dir + "/boksynk_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/boksynk_main.png"); err != nil {
		t.Fatal(err)
	}
	if err := h.TapText("Uppdatera"); err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/boksynk_list.png"); err != nil {
		t.Fatal(err)
	}
	if err := h.TapText("Synka"); err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/boksynk_synced.png"); err != nil {
		t.Fatal(err)
	}

	// The unconfigured first-launch help screen.
	h2, _, _ := bootSync(t, newLibrary("k"), nil)
	h2.TapXY(500, 700)
	if err := h2.Screenshot(dir + "/boksynk_setup.png"); err != nil {
		t.Fatal(err)
	}
}
