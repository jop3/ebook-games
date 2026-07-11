// Package store holds the pure, SDK-free logic of Spelbutiken: talking to the
// GitHub Releases API, deciding what is installed/new/outdated, and unpacking
// a downloaded game zip into the device's applications folder.
//
// Nothing here imports the inkview SDK, so it is fully unit-testable with the
// portable Go toolchain (no cgo, no device) — same layout as lasordning/series.
package store

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// maxAppBytes caps the uncompressed size of one game binary. The largest .app
// in the repo is ~15 MB; anything bigger than this is a corrupt or hostile zip.
const maxAppBytes = 64 << 20

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// Release is the subset of a GitHub release the app needs.
type Release struct {
	Tag    string
	Assets []Asset
}

// Client fetches releases and downloads assets. API/Repo/HTTP are fields so
// play tests can point the client at a local fake server.
type Client struct {
	API  string // e.g. "https://api.github.com"
	Repo string // e.g. "jop3/ebook-games"
	HTTP *http.Client
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub's API requires a User-Agent.
	req.Header.Set("User-Agent", "Spelbutiken/1.0 (PocketBook ebook-games installer)")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d från %s", resp.StatusCode, url)
	}
	return resp, nil
}

// LatestRelease fetches the newest published release and its assets.
func (c *Client) LatestRelease(ctx context.Context) (Release, error) {
	resp, err := c.get(ctx, c.API+"/repos/"+c.Repo+"/releases/latest")
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	var doc struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&doc); err != nil {
		return Release{}, fmt.Errorf("läsa svar: %w", err)
	}
	rel := Release{Tag: doc.TagName}
	for _, a := range doc.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.URL, Size: a.Size})
	}
	return rel, nil
}

// State of one game relative to what is on the device.
type State int

const (
	StateNew       State = iota // not on the device
	StateInstalled              // on the device, from this release
	StateUpdatable              // on the device but from an older/unknown release
)

// Item is one installable game in the release listing.
type Item struct {
	Name  string // "othello" (asset name minus .zip)
	Asset Asset
	State State
}

// Plan matches the release's game zips against the applications directory and
// the install manifest, producing the rows the UI lists. A .app present on
// disk but recorded under a different (or no) release tag counts as updatable:
// it may have been copied over USB long ago, so offering a fresh copy is the
// safe default.
func Plan(rel Release, appsDir string, manifest map[string]string) []Item {
	var items []Item
	for _, a := range rel.Assets {
		if !strings.HasSuffix(a.Name, ".zip") {
			continue
		}
		name := strings.TrimSuffix(a.Name, ".zip")
		it := Item{Name: name, Asset: a, State: StateNew}
		if _, err := os.Stat(filepath.Join(appsDir, name+".app")); err == nil {
			if manifest[name] == rel.Tag {
				it.State = StateInstalled
			} else {
				it.State = StateUpdatable
			}
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// Install downloads a game zip and writes its .app into appsDir. The zip is
// held in memory (a few MB), the single *.app member is extracted to a
// temporary file and renamed into place, so a failed download can never leave
// a half-written binary behind — and renaming over a running app (updating
// Spelbutiken itself) is safe on Linux.
func (c *Client) Install(ctx context.Context, a Asset, appsDir string) error {
	resp, err := c.get(ctx, a.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAppBytes))
	if err != nil {
		return fmt.Errorf("hämta %s: %w", a.Name, err)
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("öppna zip %s: %w", a.Name, err)
	}
	for _, f := range zr.File {
		// Ignore any directory structure inside the zip; we only want the
		// single <game>.app member, written under its base name.
		base := path.Base(f.Name)
		if !strings.HasSuffix(base, ".app") || f.FileInfo().IsDir() {
			continue
		}
		if f.UncompressedSize64 > maxAppBytes {
			return fmt.Errorf("%s: orimligt stor (%d byte)", base, f.UncompressedSize64)
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxAppBytes))
		rc.Close()
		if err != nil {
			return err
		}
		tmp := filepath.Join(appsDir, base+".tmp")
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, filepath.Join(appsDir, base)); err != nil {
			os.Remove(tmp)
			return err
		}
		return nil
	}
	return fmt.Errorf("%s: ingen .app i zipfilen", a.Name)
}

// --- install manifest ---------------------------------------------------------

// LoadManifest reads the game -> release-tag record kept next to the binary.
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
