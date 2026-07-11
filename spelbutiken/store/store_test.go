package store

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// gameZip builds an in-memory release zip containing one <name>.app member.
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

// fakeGitHub serves a latest-release document and its assets.
func fakeGitHub(t *testing.T, tag string, zips map[string][]byte) (*httptest.Server, *Client) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	type asset struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	}
	doc := struct {
		TagName string  `json:"tag_name"`
		Assets  []asset `json:"assets"`
	}{TagName: tag}
	for name, b := range zips {
		doc.Assets = append(doc.Assets, asset{
			Name: name, URL: srv.URL + "/dl/" + name, Size: int64(len(b)),
		})
	}
	mux.HandleFunc("/repos/jop3/ebook-games/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "User-Agent required", http.StatusForbidden)
			return
		}
		json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		b, ok := zips[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	})
	return srv, &Client{API: srv.URL, Repo: "jop3/ebook-games", HTTP: srv.Client()}
}

func TestLatestReleaseAndPlan(t *testing.T) {
	zips := map[string][]byte{
		"othello.zip":        gameZip(t, "othello", []byte("elf-othello")),
		"sudoku.zip":         gameZip(t, "sudoku", []byte("elf-sudoku")),
		"spelbutiken.instal": []byte("raw"), // non-zip asset must be skipped
	}
	_, c := fakeGitHub(t, "v1.2.0", zips)

	rel, err := c.LatestRelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v1.2.0" || len(rel.Assets) != 3 {
		t.Fatalf("rel=%+v", rel)
	}

	dir := t.TempDir()
	// sudoku already on disk but from an unknown release -> updatable.
	if err := os.WriteFile(filepath.Join(dir, "sudoku.app"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	items := Plan(rel, dir, map[string]string{})
	if len(items) != 2 {
		t.Fatalf("non-zip asset should be excluded, items=%+v", items)
	}
	if items[0].Name != "othello" || items[0].State != StateNew {
		t.Fatalf("othello should be new: %+v", items[0])
	}
	if items[1].Name != "sudoku" || items[1].State != StateUpdatable {
		t.Fatalf("sudoku (on disk, unknown tag) should be updatable: %+v", items[1])
	}

	// Recorded under the current tag -> installed.
	items = Plan(rel, dir, map[string]string{"sudoku": "v1.2.0"})
	if items[1].State != StateInstalled {
		t.Fatalf("sudoku recorded at v1.2.0 should be installed: %+v", items[1])
	}
}

func TestInstallWritesExecutableAtomically(t *testing.T) {
	zips := map[string][]byte{"othello.zip": gameZip(t, "othello", []byte("elf-bytes"))}
	_, c := fakeGitHub(t, "v1.0.0", zips)
	rel, err := c.LatestRelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := c.Install(context.Background(), rel.Assets[0], dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "othello.app")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed app must be executable, mode=%v", st.Mode())
	}
	b, _ := os.ReadFile(path)
	if string(b) != "elf-bytes" {
		t.Fatalf("wrong content: %q", b)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file must not remain after install")
	}
}

func TestInstallRejectsZipWithoutApp(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("README.txt")
	w.Write([]byte("not a game"))
	zw.Close()
	zips := map[string][]byte{"broken.zip": buf.Bytes()}
	_, c := fakeGitHub(t, "v1.0.0", zips)
	rel, _ := c.LatestRelease(context.Background())

	dir := t.TempDir()
	if err := c.Install(context.Background(), rel.Assets[0], dir); err == nil {
		t.Fatal("zip without a .app member must be rejected")
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 0 {
		t.Fatalf("nothing may be written on failure, got %v", ents)
	}
}

func TestInstallStripsZipDirectories(t *testing.T) {
	// A zip whose .app sits inside a folder (and tries to escape) must still
	// land as a plain file in the target dir.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("../nested/dir/evil.app")
	w.Write([]byte("x"))
	zw.Close()
	zips := map[string][]byte{"evil.zip": buf.Bytes()}
	_, c := fakeGitHub(t, "v1.0.0", zips)
	rel, _ := c.LatestRelease(context.Background())

	dir := t.TempDir()
	if err := c.Install(context.Background(), rel.Assets[0], dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.app")); err != nil {
		t.Fatal("app must be written under its base name inside the target dir")
	}
}

func TestManifestRoundTripAndCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.json")
	if got := LoadManifest(path); len(got) != 0 {
		t.Fatalf("missing manifest should load empty, got %v", got)
	}
	want := map[string]string{"othello": "v1.0.0"}
	if err := SaveManifest(path, want); err != nil {
		t.Fatal(err)
	}
	if got := LoadManifest(path); got["othello"] != "v1.0.0" {
		t.Fatalf("round trip failed: %v", got)
	}
	os.WriteFile(path, []byte("{broken"), 0o644)
	if got := LoadManifest(path); len(got) != 0 {
		t.Fatalf("corrupt manifest should load empty, got %v", got)
	}
}
