package drive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// --- FolderID ---------------------------------------------------------------------

func TestFolderID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://drive.google.com/drive/folders/1AbC_dEf-123456789?usp=sharing", "1AbC_dEf-123456789"},
		{"https://drive.google.com/drive/u/0/folders/1AbC_dEf-123456789", "1AbC_dEf-123456789"},
		{"https://drive.google.com/open?id=1AbC_dEf-123456789", "1AbC_dEf-123456789"},
		{"1AbC_dEf-123456789", "1AbC_dEf-123456789"},
		{"  1AbC_dEf-123456789  ", "1AbC_dEf-123456789"},
		{"https://drive.google.com/drive/my-drive", ""},
		{"not an id", ""},
		{"", ""},
		{"<klistra in delningslänken till din Drive-mapp här>", ""},
	}
	for _, c := range cases {
		if got := FolderID(c.in); got != c.want {
			t.Errorf("FolderID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- config -----------------------------------------------------------------------

func TestConfigReadyAndTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "boksynk.json")

	// Missing file: empty config, no error.
	c, err := LoadConfig(path)
	if err != nil || c.Ready() {
		t.Fatalf("missing config should be empty and not ready: %v %v", c, err)
	}

	// The template is not Ready (placeholders) and is never overwritten.
	if err := WriteTemplate(path); err != nil {
		t.Fatal(err)
	}
	c, err = LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Ready() {
		t.Fatal("template placeholders must not count as configured")
	}
	if err := os.WriteFile(path, []byte(`{"folder":"1AbC_dEf-123456789","apiKey":"k"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteTemplate(path); err != nil {
		t.Fatal(err)
	}
	c, _ = LoadConfig(path)
	if !c.Ready() {
		t.Fatal("WriteTemplate must not overwrite a filled-in config")
	}

	// Malformed JSON surfaces an error for the UI.
	if err := os.WriteFile(path, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("malformed config should return an error")
	}
}

func TestIsBook(t *testing.T) {
	c := Config{}
	for _, name := range []string{"Dune.epub", "DUNE.EPUB", "a.pdf", "b.fb2.zip", "x.mobi", "y.cbz"} {
		if !c.IsBook(name) {
			t.Errorf("%s should be a book", name)
		}
	}
	for _, name := range []string{"cover.jpg", "notes.json", "archive.zip", "app.exe"} {
		if c.IsBook(name) {
			t.Errorf("%s should NOT be a book", name)
		}
	}
	c.Extensions = []string{"epub"} // no dot, mixed usage
	if !c.IsBook("a.epub") || c.IsBook("a.pdf") {
		t.Fatal("custom extension list should be honored (and dot-normalized)")
	}
}

func TestSafeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Dune.epub", "Dune.epub"},
		{"a/b.epub", "a_b.epub"},
		{`a\b.epub`, `a_b.epub`},
		{"..", "_.."},
		{".", "_."},
		{"  x.epub  ", "x.epub"},
	}
	for _, c := range cases {
		if got := safeName(c.in); got != c.want {
			t.Errorf("safeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- fake Drive server ------------------------------------------------------------

type fakeFile struct {
	id, name, parent, mime string
	body                   []byte
}

func md5hex(b []byte) string {
	h := md5.Sum(b)
	return hex.EncodeToString(h[:])
}

// newFakeDrive serves a minimal files.list + alt=media API for the given tree.
func newFakeDrive(t *testing.T, files []fakeFile, wantKey string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != wantKey {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "API key not valid"}})
			return
		}
		q := r.URL.Query().Get("q")
		// q = '<id>' in parents and trashed=false
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
		// One file per page to exercise pagination.
		var in []jf
		for _, f := range files {
			if f.parent != parent {
				continue
			}
			e := jf{ID: f.id, Name: f.name, MimeType: f.mime}
			if f.mime != folderMime {
				e.Size = strconv.Itoa(len(f.body))
				e.MD5 = md5hex(f.body)
			}
			in = append(in, e)
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
		if r.URL.Query().Get("alt") != "media" {
			http.Error(w, "expected alt=media", http.StatusBadRequest)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/files/")
		for _, f := range files {
			if f.id == id {
				w.Write(f.body)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "File not found"}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// rootID is ID-shaped so Config{Folder: rootID} passes FolderID validation.
const rootID = "root123456789"

func testTree() []fakeFile {
	return []fakeFile{
		{id: rootID, name: "Böcker", mime: folderMime},
		{id: "f1", name: "Dune.epub", parent: rootID, body: []byte("dune-epub-bytes")},
		{id: "f2", name: "cover.jpg", parent: rootID, body: []byte("not-a-book")},
		{id: "sub", name: "Serier", parent: rootID, mime: folderMime},
		{id: "f3", name: "Foundation.epub", parent: "sub", body: []byte("foundation-bytes")},
		{id: "f4", name: "Manual.pdf", parent: rootID, body: []byte("pdf-bytes")},
	}
}

func newClient(srv *httptest.Server, key string) *Client {
	return &Client{API: srv.URL, Key: key, HTTP: srv.Client()}
}

// --- listing ----------------------------------------------------------------------

func TestListFolder(t *testing.T) {
	srv := newFakeDrive(t, testTree(), "k")
	c := newClient(srv, "k")
	cfg := Config{Folder: rootID, APIKey: "k"}

	files, err := c.ListFolder(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, f := range files {
		rels = append(rels, f.RelPath)
	}
	want := []string{"Dune.epub", "Manual.pdf", "Serier/Foundation.epub"}
	if strings.Join(rels, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", rels, want)
	}
	for _, f := range files {
		if f.MD5 == "" || f.Size == 0 {
			t.Fatalf("size/md5 should be parsed: %+v", f)
		}
	}
}

func TestListFolderBadKey(t *testing.T) {
	srv := newFakeDrive(t, testTree(), "good")
	c := newClient(srv, "bad")
	_, err := c.ListFolder(context.Background(), Config{Folder: rootID, APIKey: "bad"})
	if err == nil || !strings.Contains(err.Error(), "API-nyckeln") {
		t.Fatalf("a 403 should carry the API key hint, got: %v", err)
	}
}

// --- plan + download --------------------------------------------------------------

func TestPlanAndDownloadFlow(t *testing.T) {
	srv := newFakeDrive(t, testTree(), "k")
	c := newClient(srv, "k")
	cfg := Config{Folder: rootID, APIKey: "k"}

	files, err := c.ListFolder(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	manifest := map[string]string{}

	// Everything starts as new.
	items := Plan(files, dir, manifest)
	for _, it := range items {
		if it.State != StateNew {
			t.Fatalf("%s should be new, got %v", it.File.RelPath, it.State)
		}
	}

	// Download all; subfolder must be created; manifest updated.
	for _, it := range items {
		sum, err := c.Download(context.Background(), it.File, dir)
		if err != nil {
			t.Fatal(err)
		}
		if sum != it.File.MD5 {
			t.Fatalf("returned md5 %s != remote %s", sum, it.File.MD5)
		}
		manifest[it.File.RelPath] = sum
	}
	b, err := os.ReadFile(filepath.Join(dir, "Serier", "Foundation.epub"))
	if err != nil || string(b) != "foundation-bytes" {
		t.Fatalf("subfolder book should exist with content: %v %q", err, b)
	}

	// Re-plan: all synced.
	for _, it := range Plan(files, dir, manifest) {
		if it.State != StateSynced {
			t.Fatalf("%s should be synced, got %v", it.File.RelPath, it.State)
		}
	}

	// A book that changed on Drive (different md5) becomes Changed.
	changed := files
	changed[0].MD5 = md5hex([]byte("new-revision"))
	if got := Plan(changed, dir, manifest)[0].State; got != StateChanged {
		t.Fatalf("remote-edited book should be Changed, got %v", got)
	}

	// An identical book copied over USB (no manifest entry) is recognized
	// by its real MD5 — never downloaded twice.
	files2, _ := c.ListFolder(context.Background(), cfg)
	for _, it := range Plan(files2, dir, map[string]string{}) {
		if it.State != StateSynced {
			t.Fatalf("identical local file should be Synced without manifest, got %v for %s", it.State, it.File.RelPath)
		}
	}
}

func TestDownloadVerifiesChecksumAndIsAtomic(t *testing.T) {
	tree := []fakeFile{
		{id: rootID, name: "B", mime: folderMime},
		{id: "f1", name: "Dune.epub", parent: rootID, body: []byte("dune")},
	}
	srv := newFakeDrive(t, tree, "k")
	c := newClient(srv, "k")
	dir := t.TempDir()

	// Corrupt transfer: remote metadata says another md5.
	f := RemoteFile{ID: "f1", Name: "Dune.epub", RelPath: "Dune.epub", Size: 4, MD5: md5hex([]byte("other"))}
	if _, err := c.Download(context.Background(), f, dir); err == nil {
		t.Fatal("md5 mismatch must fail the download")
	}
	if _, err := os.Stat(filepath.Join(dir, "Dune.epub")); !os.IsNotExist(err) {
		t.Fatal("a failed download must not leave a book behind")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("no temp litter allowed, found %v", entries)
	}

	// Oversized metadata is refused before transfer.
	f.Size = maxBookBytes + 1
	if _, err := c.Download(context.Background(), f, dir); err == nil {
		t.Fatal("oversized file must be refused")
	}
}

// --- manifest ---------------------------------------------------------------------

func TestManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.json")
	if got := LoadManifest(path); len(got) != 0 {
		t.Fatal("missing manifest should be empty")
	}
	m := map[string]string{"a.epub": "123"}
	if err := SaveManifest(path, m); err != nil {
		t.Fatal(err)
	}
	if got := LoadManifest(path); got["a.epub"] != "123" {
		t.Fatalf("round trip failed: %v", got)
	}
	os.WriteFile(path, []byte("garbage"), 0o644)
	if got := LoadManifest(path); len(got) != 0 {
		t.Fatal("corrupt manifest should load as empty")
	}
}
