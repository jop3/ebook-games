package story

// Save / restore (spec §5). The whole State (§3.3) is serialized to a single
// slot with encoding/gob behind a version guard. This lives in the ink-free
// package so it round-trips in unit tests without a device. A corrupt or
// out-of-version save is reported as an error so the caller can fall back to a
// fresh game instead of crashing.

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"os"
)

// saveVersion is bumped whenever the State layout changes so old saves are
// rejected rather than mis-decoded.
const saveVersion byte = 1

// ErrBadSave indicates a missing, corrupt, or wrong-version save.
var ErrBadSave = errors.New("story: unrecognised or out-of-date save")

// Save writes s to w: a one-byte version tag followed by a gob-encoded State.
func Save(w io.Writer, s *State) error {
	if _, err := w.Write([]byte{saveVersion}); err != nil {
		return err
	}
	return gob.NewEncoder(w).Encode(s)
}

// Load reads a State previously written by Save. A version mismatch or decode
// failure returns ErrBadSave.
func Load(r io.Reader) (*State, error) {
	var ver [1]byte
	if _, err := io.ReadFull(r, ver[:]); err != nil {
		return nil, ErrBadSave
	}
	if ver[0] != saveVersion {
		return nil, ErrBadSave
	}
	var s State
	if err := gob.NewDecoder(r).Decode(&s); err != nil {
		return nil, ErrBadSave
	}
	normalize(&s)
	return &s, nil
}

// SaveBytes serializes s to a byte slice (convenient for tests and atomic writes).
func SaveBytes(s *State) ([]byte, error) {
	var buf bytes.Buffer
	if err := Save(&buf, s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LoadBytes deserializes a State from bytes produced by SaveBytes.
func LoadBytes(b []byte) (*State, error) {
	return Load(bytes.NewReader(b))
}

// SaveFile writes the save to path atomically (via a temp file + rename) so a
// crash mid-write never corrupts the existing slot.
func SaveFile(path string, s *State) error {
	b, err := SaveBytes(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadFile reads a save from path; a missing/corrupt/old file returns ErrBadSave.
func LoadFile(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrBadSave
	}
	return LoadBytes(b)
}

// normalize re-initialises any nil map so a loaded State is structurally
// identical to a freshly-built one (gob decodes empty maps as nil).
func normalize(s *State) {
	if s.Visited == nil {
		s.Visited = map[LocID]bool{}
	}
	if s.Carried == nil {
		s.Carried = map[ObjID]bool{}
	}
	if s.ObjAt == nil {
		s.ObjAt = map[ObjID]LocID{}
	}
	if s.ObjState == nil {
		s.ObjState = map[ObjID]int{}
	}
	if s.Known == nil {
		s.Known = map[string]bool{}
	}
	if s.Deductions == nil {
		s.Deductions = map[string]bool{}
	}
}
