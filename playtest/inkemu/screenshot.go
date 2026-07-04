package ink

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

// Screenshot writes the current framebuffer to a PNG at path (creating parent
// dirs). Useful for eyeballing that a headless playthrough "makes sense".
func (h *Harness) Screenshot(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, h.Frame()); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}
