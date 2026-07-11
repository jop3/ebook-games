// Package drive holds the pure, SDK-free logic of Boksynk: reading the user's
// config, talking to the Google Drive API (v3), deciding which books are
// new/changed/already synced, and downloading them atomically into the
// device's books folder.
//
// Access model: the user shares their Drive book folder as "anyone with the
// link" and creates a free Google API key (one-time setup on a computer).
// With that, the folder can be listed and its files downloaded with plain
// HTTPS GETs — no OAuth dance, which a browserless e-ink device can't do
// (Google's device flow does not allow the drive.readonly scope).
//
// Nothing here imports the inkview SDK, so it is fully unit-testable with the
// portable Go toolchain (no cgo, no device) — same layout as spelbutiken/store.
package drive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// maxBookBytes caps one downloaded file. Big illustrated PDFs can reach a few
// hundred MB; anything past this is a mistake, not a book.
const maxBookBytes = 1 << 30 // 1 GiB

// maxDepth bounds the subfolder recursion so a cyclic or absurdly deep share
// can't hang the listing.
const maxDepth = 6

// folderMime is the Drive mimeType marking a subfolder.
const folderMime = "application/vnd.google-apps.folder"

// --- config -----------------------------------------------------------------------

// DefaultBooksDir is where synced books land on the device. It sits under the
// folder the PocketBook library scans, in its own subfolder so synced books
// never mix with (or overwrite) anything copied over USB.
const DefaultBooksDir = "/mnt/ext1/Books/Drive"

// DefaultExtensions are the book formats the PocketBook reads natively.
// Matching is a case-insensitive suffix test against these.
var DefaultExtensions = []string{
	".epub", ".pdf", ".fb2", ".fb2.zip", ".mobi", ".azw", ".azw3",
	".txt", ".rtf", ".djvu", ".doc", ".docx", ".html", ".htm", ".cbz", ".cbr",
}

// Config is the user's one-time setup, kept as a small JSON file next to the
// app binary (edited over USB or with KOReader's text editor).
type Config struct {
	// Folder is the Google Drive share link of the book folder — or just its
	// raw folder ID. The folder must be shared as "anyone with the link".
	Folder string `json:"folder"`
	// APIKey is a Google API key with the Drive API enabled.
	APIKey string `json:"apiKey"`
	// BooksDir overrides where books are written (default DefaultBooksDir).
	BooksDir string `json:"booksDir,omitempty"`
	// Extensions overrides which file types count as books.
	Extensions []string `json:"extensions,omitempty"`
}

// Ready reports whether the config has both required fields filled in
// (ignoring the placeholders the template writes).
func (c Config) Ready() bool {
	return FolderID(c.Folder) != "" && c.APIKey != "" && !strings.HasPrefix(c.APIKey, "<")
}

// Dir returns the effective books directory.
func (c Config) Dir() string {
	if c.BooksDir != "" {
		return c.BooksDir
	}
	return DefaultBooksDir
}

// exts returns the effective extension list, lower-cased.
func (c Config) exts() []string {
	src := c.Extensions
	if len(src) == 0 {
		src = DefaultExtensions
	}
	out := make([]string, len(src))
	for i, e := range src {
		e = strings.ToLower(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		out[i] = e
	}
	return out
}

// IsBook reports whether a file name matches the configured book formats.
func (c Config) IsBook(name string) bool {
	n := strings.ToLower(name)
	for _, e := range c.exts() {
		if strings.HasSuffix(n, e) {
			return true
		}
	}
	return false
}

// LoadConfig reads the config file. A missing file returns an empty (not
// Ready) config and no error; a malformed file returns the error so the UI
// can show it instead of silently doing nothing.
func LoadConfig(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return c, nil
}

// WriteTemplate creates a fill-in-the-blanks config file so first-run setup is
// "plug in USB, open one file, paste two values". Never overwrites.
func WriteTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	c := Config{
		Folder: "<klistra in delningslänken till din Drive-mapp här>",
		APIKey: "<klistra in din Google API-nyckel här>",
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// reFolderID matches a plausible Drive file/folder ID.
var reFolderID = regexp.MustCompile(`^[A-Za-z0-9_-]{10,}$`)

// FolderID extracts the folder ID from a Drive share link
// (https://drive.google.com/drive/folders/<ID>?usp=sharing, the u/0 variant,
// or an ?id=<ID> form) — or returns the input itself if it already looks like
// a raw ID. Returns "" if nothing ID-like is found.
func FolderID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		if id := u.Query().Get("id"); reFolderID.MatchString(id) {
			return id
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, p := range parts {
			if p == "folders" && i+1 < len(parts) && reFolderID.MatchString(parts[i+1]) {
				return parts[i+1]
			}
		}
		return ""
	}
	if reFolderID.MatchString(s) {
		return s
	}
	return ""
}

// --- Drive API client -------------------------------------------------------------

// RemoteFile is one book in the Drive folder (flattened across subfolders).
type RemoteFile struct {
	ID      string
	Name    string // base file name
	RelPath string // path under the books dir, subfolders preserved ("Serier/Dune.epub")
	Size    int64
	MD5     string // empty for Google-native files (which we skip anyway)
}

// Client lists and downloads from the Drive API. API/HTTP are fields so tests
// can point it at a local fake server.
type Client struct {
	API  string // e.g. "https://www.googleapis.com/drive/v3"
	Key  string
	HTTP *http.Client
}

func (c *Client) get(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Boksynk/1.0 (PocketBook Drive book sync)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, apiError(resp)
	}
	return resp, nil
}

// apiError turns a non-200 Drive response into a message the status line can
// show, with a hint for the two setup mistakes everyone makes.
func apiError(resp *http.Response) error {
	var doc struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	msg := ""
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc) == nil {
		msg = doc.Error.Message
	}
	hint := ""
	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusBadRequest, http.StatusUnauthorized:
		hint = " — kontrollera API-nyckeln (och att Drive API är aktiverat)"
	case http.StatusNotFound:
		hint = " — kontrollera mappens delningslänk (måste vara »alla med länken«)"
	}
	if msg == "" {
		return fmt.Errorf("HTTP %d%s", resp.StatusCode, hint)
	}
	return fmt.Errorf("HTTP %d: %s%s", resp.StatusCode, msg, hint)
}

// ListFolder walks the shared folder (and its subfolders, breadth-first) and
// returns every file matching the config's book formats, sorted by RelPath.
func (c *Client) ListFolder(ctx context.Context, cfg Config) ([]RemoteFile, error) {
	root := FolderID(cfg.Folder)
	if root == "" {
		return nil, fmt.Errorf("ingen giltig mapp i konfigurationen")
	}

	type dir struct {
		id, rel string
		depth   int
	}
	queue := []dir{{id: root}}
	seen := map[string]bool{root: true}
	var out []RemoteFile

	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]

		pageToken := ""
		for {
			q := url.Values{
				"q":                         {"'" + d.id + "' in parents and trashed=false"},
				"fields":                    {"nextPageToken,files(id,name,mimeType,size,md5Checksum)"},
				"pageSize":                  {"200"},
				"key":                       {c.Key},
				"supportsAllDrives":         {"true"},
				"includeItemsFromAllDrives": {"true"},
			}
			if pageToken != "" {
				q.Set("pageToken", pageToken)
			}
			resp, err := c.get(ctx, c.API+"/files?"+q.Encode())
			if err != nil {
				return nil, err
			}
			var doc struct {
				NextPageToken string `json:"nextPageToken"`
				Files         []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					MimeType string `json:"mimeType"`
					Size     string `json:"size"` // the API sends int64 as a string
					MD5      string `json:"md5Checksum"`
				} `json:"files"`
			}
			err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&doc)
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("läsa fillistan: %w", err)
			}

			for _, f := range doc.Files {
				if f.MimeType == folderMime {
					if d.depth+1 <= maxDepth && !seen[f.ID] {
						seen[f.ID] = true
						queue = append(queue, dir{id: f.ID, rel: joinRel(d.rel, safeName(f.Name)), depth: d.depth + 1})
					}
					continue
				}
				if !cfg.IsBook(f.Name) {
					continue
				}
				size, _ := strconv.ParseInt(f.Size, 10, 64)
				name := safeName(f.Name)
				out = append(out, RemoteFile{
					ID:      f.ID,
					Name:    name,
					RelPath: joinRel(d.rel, name),
					Size:    size,
					MD5:     f.MD5,
				})
			}
			pageToken = doc.NextPageToken
			if pageToken == "" {
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

func joinRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// safeName strips path separators and dot-dot tricks from a Drive file name so
// a hostile share can't write outside the books dir.
func safeName(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.TrimSpace(s)
	for s == "." || s == ".." || s == "" {
		s = "_" + s
	}
	return s
}

// --- sync planning -----------------------------------------------------------------

// State of one remote book relative to the device.
type State int

const (
	StateNew     State = iota // not on the device yet
	StateChanged              // on the device but the Drive copy differs
	StateSynced               // identical copy already on the device
)

// Item is one row in the sync listing.
type Item struct {
	File  RemoteFile
	State State
}

// Plan compares the remote listing against the books dir and the sync
// manifest. The manifest remembers the MD5 of what we downloaded; if it is
// missing or stale (e.g. the book was copied over USB), the local file's real
// MD5 decides, so an identical book is never downloaded twice.
func Plan(files []RemoteFile, booksDir string, manifest map[string]string) []Item {
	items := make([]Item, 0, len(files))
	for _, f := range files {
		it := Item{File: f, State: StateNew}
		local := filepath.Join(booksDir, filepath.FromSlash(f.RelPath))
		if st, err := os.Stat(local); err == nil && !st.IsDir() {
			switch {
			case f.MD5 != "" && manifest[f.RelPath] == f.MD5:
				it.State = StateSynced
			case f.MD5 != "" && fileMD5(local) == f.MD5:
				it.State = StateSynced
			case f.MD5 == "" && st.Size() == f.Size:
				it.State = StateSynced // no checksum from the API; size is all we have
			default:
				it.State = StateChanged
			}
		}
		items = append(items, it)
	}
	return items
}

func fileMD5(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Download fetches one book into booksDir, streaming to a temp file and
// renaming into place so an interrupted transfer never leaves a torn book in
// the library. Returns the MD5 recorded in the manifest.
func (c *Client) Download(ctx context.Context, f RemoteFile, booksDir string) (string, error) {
	if f.Size > maxBookBytes {
		return "", fmt.Errorf("%s: orimligt stor (%d byte)", f.Name, f.Size)
	}
	u := c.API + "/files/" + url.PathEscape(f.ID) + "?" + url.Values{
		"alt":               {"media"},
		"key":               {c.Key},
		"supportsAllDrives": {"true"},
	}.Encode()
	resp, err := c.get(ctx, u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	local := filepath.Join(booksDir, filepath.FromSlash(f.RelPath))
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return "", err
	}
	tmp := local + ".boksynk-tmp"
	w, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := md5.New()
	_, err = io.Copy(io.MultiWriter(w, h), io.LimitReader(resp.Body, maxBookBytes+1))
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("hämta %s: %w", f.Name, err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if f.MD5 != "" && got != f.MD5 {
		os.Remove(tmp)
		return "", fmt.Errorf("%s: nedladdningen blev skadad (fel kontrollsumma)", f.Name)
	}
	if err := os.Rename(tmp, local); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return got, nil
}

// --- sync manifest -----------------------------------------------------------------

// LoadManifest reads the relpath -> md5 record kept next to the binary.
// A missing or corrupt file is an empty manifest, so first run works.
func LoadManifest(path string) map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	var got map[string]string
	if json.Unmarshal(b, &got) == nil && got != nil {
		m = got
	}
	return m
}

// SaveManifest writes the manifest atomically.
func SaveManifest(path string, m map[string]string) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
