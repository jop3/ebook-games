//go:build playtest

package main

// Headless play tests for Spelbutiken under the pure-Go inkview emulator
// (playtest/play.sh spelbutiken). A local httptest server stands in for the
// GitHub API, and the app is pointed at a temp applications directory, so the
// whole install flow — fetch the release listing, tap a game, download,
// unpack, atomic write, manifest update, row state change — runs through the
// real touch UI with no network.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	ink "github.com/dennwc/inkview"

	"spelbutiken/store"
)

func gameZip(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name + ".app")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// bootShop boots the app against a fake release server and a temp
// applications dir. fail makes every endpoint return HTTP 500.
func bootShop(t *testing.T, tag string, zips map[string][]byte, fail bool) (*ink.Harness, *app) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	type asset struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	}
	mux.HandleFunc("/repos/jop3/ebook-games/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		doc := struct {
			TagName string  `json:"tag_name"`
			Assets  []asset `json:"assets"`
		}{TagName: tag}
		for name, b := range zips {
			doc.Assets = append(doc.Assets, asset{Name: name, URL: srv.URL + "/dl/" + name, Size: int64(len(b))})
		}
		json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) {
		if b, ok := zips[filepath.Base(r.URL.Path)]; ok {
			w.Write(b)
			return
		}
		http.NotFound(w, r)
	})

	dir := t.TempDir()
	a := &app{
		client:  &store.Client{API: srv.URL, Repo: "jop3/ebook-games", HTTP: srv.Client()},
		appsDir: dir,
		manPath: filepath.Join(dir, "manifest.json"),
	}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	return h, a
}

// --- fetch the listing --------------------------------------------------------

func TestPlaySpelbutikenFetchAndList(t *testing.T) {
	h, a := bootShop(t, "v1.0.0", map[string][]byte{
		"othello.zip":         gameZip(t, "othello", []byte("elf-o")),
		"sudoku.zip":          gameZip(t, "sudoku", []byte("elf-s")),
		"spelbutiken.install": []byte("raw-bootstrap"), // must not become a row
	}, false)

	if _, ok := h.FindText("Spelbutiken"); !ok {
		t.Fatal("title should be visible")
	}
	if err := h.TapText("Hämta listan"); err != nil {
		t.Fatal(err)
	}
	if !a.fetched || len(a.items) != 2 {
		t.Fatalf("expected 2 game rows after fetch, items=%v", a.items)
	}
	if _, ok := h.FindTextContains("Utgåva v1.0.0"); !ok {
		t.Fatal("the release tag should be shown after fetching")
	}
	if _, ok := h.FindText("othello"); !ok {
		t.Fatal("game rows should be listed")
	}
	if _, ok := h.FindTextContains("spelbutiken.install"); ok {
		t.Fatal("the raw bootstrap asset must not be listed as a game")
	}
}

// --- install one game via a real tap -------------------------------------------

func TestPlaySpelbutikenInstallOne(t *testing.T) {
	h, a := bootShop(t, "v1.0.0", map[string][]byte{
		"othello.zip": gameZip(t, "othello", []byte("elf-o")),
	}, false)
	if err := h.TapText("Hämta listan"); err != nil {
		t.Fatal(err)
	}

	// Tap the row.
	if len(a.rowRects) != 1 {
		t.Fatalf("expected 1 row rect, got %d", len(a.rowRects))
	}
	if !h.TapRect(a.rowRects[0]) {
		t.Fatal("tapping a game row should be handled")
	}

	// The binary must be on "the device", executable, and recorded.
	st, err := os.Stat(filepath.Join(a.appsDir, "othello.app"))
	if err != nil {
		t.Fatal("othello.app should have been installed:", err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed app must be executable, mode=%v", st.Mode())
	}
	if got := store.LoadManifest(a.manPath); got["othello"] != "v1.0.0" {
		t.Fatalf("manifest should record the release, got %v", got)
	}
	if a.items[0].State != store.StateInstalled {
		t.Fatalf("row should now be installed, state=%v", a.items[0].State)
	}
	if _, ok := h.FindTextContains("installerad"); !ok {
		t.Fatal("the row should display its installed state")
	}
}

// --- Installera allt skips what is already current ------------------------------

func TestPlaySpelbutikenInstallAll(t *testing.T) {
	h, a := bootShop(t, "v2.0.0", map[string][]byte{
		"othello.zip": gameZip(t, "othello", []byte("elf-o")),
		"sudoku.zip":  gameZip(t, "sudoku", []byte("elf-s")),
		"hex.zip":     gameZip(t, "hex", []byte("elf-h")),
	}, false)

	// hex is already on disk from an older, unrecorded install (USB copy).
	if err := os.WriteFile(filepath.Join(a.appsDir, "hex.app"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := h.TapText("Hämta listan"); err != nil {
		t.Fatal(err)
	}
	if a.items[0].Name != "hex" || a.items[0].State != store.StateUpdatable {
		t.Fatalf("a stale on-disk app should be updatable: %+v", a.items[0])
	}

	if err := h.TapText("Installera allt"); err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{"othello", "sudoku", "hex"} {
		b, err := os.ReadFile(filepath.Join(a.appsDir, g+".app"))
		if err != nil {
			t.Fatalf("%s should be installed: %v", g, err)
		}
		if string(b) == "old" {
			t.Fatal("the stale hex.app should have been replaced")
		}
	}
	for _, it := range a.items {
		if it.State != store.StateInstalled {
			t.Fatalf("all rows should be installed, %s=%v", it.Name, it.State)
		}
	}

	// A second Installera allt is a no-op.
	if err := h.TapText("Installera allt"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.FindTextContains("redan installerat"); !ok {
		t.Fatal("re-running install-all should report nothing to do")
	}
}

// --- network failure fails soft --------------------------------------------------

func TestPlaySpelbutikenFetchFailure(t *testing.T) {
	h, a := bootShop(t, "v1.0.0", nil, true)
	if err := h.TapText("Hämta listan"); err != nil {
		t.Fatal(err)
	}
	if a.fetched {
		t.Fatal("a failed fetch must not mark the listing as loaded")
	}
	if _, ok := h.FindTextContains("Kunde inte hämta listan"); !ok {
		t.Fatal("the error should be reported on screen")
	}
	// The app must stay alive and re-tappable.
	if err := h.TapText("Hämta listan"); err != nil {
		t.Fatal(err)
	}
}
