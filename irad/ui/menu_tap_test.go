package ui

import (
	"image"
	"testing"

	"irad/game"
)

// TestMenuTapHitsPresetRows simulates the exact flow that happens on device:
// Draw() populates the hit rectangles, then a tap at a preset row's center
// must resolve to that preset. This is a headless reproduction of the on-device
// interaction to catch coordinate/hit-testing bugs without the hardware.
func TestMenuTapHitsPresetRows(t *testing.T) {
	m := NewMenu()
	f := &Fonts{} // stub fonts; Draw only needs them for SetActive (no-op in stub)
	screen := image.Pt(1072, 1448)

	// Draw populates presetRows, modeBtn, steppers, etc.
	m.Draw(screen, f)

	if len(m.presetRows) != len(game.Presets) {
		t.Fatalf("expected %d preset rows after Draw, got %d", len(game.Presets), len(m.presetRows))
	}

	// Tap the center of each preset row; it must return that preset.
	for i, row := range m.presetRows {
		center := image.Pt(
			(row.rect.Min.X+row.rect.Max.X)/2,
			(row.rect.Min.Y+row.rect.Max.Y)/2,
		)
		act := m.HandleTouch(center)
		if act.StartPreset == nil {
			t.Errorf("row %d (%s) center %v did not start a preset (rect=%v)", i, row.label, center, row.rect)
			continue
		}
		if act.StartPreset.Name != game.Presets[i].Name {
			t.Errorf("row %d: tapped %s but got preset %s", i, row.label, act.StartPreset.Name)
		}
	}
}

// TestMenuTapModeButton verifies the mode selector cycles on tap.
func TestMenuTapModeButton(t *testing.T) {
	m := NewMenu()
	f := &Fonts{}
	screen := image.Pt(1072, 1448)
	m.Draw(screen, f)

	center := image.Pt(
		(m.modeBtn.Min.X+m.modeBtn.Max.X)/2,
		(m.modeBtn.Min.Y+m.modeBtn.Max.Y)/2,
	)
	before := m.Mode
	act := m.HandleTouch(center)
	if !act.Redraw {
		t.Fatalf("tapping mode button at %v did not request redraw (modeBtn=%v)", center, m.modeBtn)
	}
	if m.Mode == before {
		t.Fatalf("mode did not change on tap")
	}
}

// TestMenuElementsOnScreen asserts every interactive rectangle lies fully
// within the 1072x1448 screen — catches off-screen layout like Einstein had.
func TestMenuElementsOnScreen(t *testing.T) {
	m := NewMenu()
	f := &Fonts{}
	screen := image.Rect(0, 0, 1072, 1448)
	m.Draw(image.Pt(1072, 1448), f)

	check := func(name string, r image.Rectangle) {
		if !r.In(screen) {
			t.Errorf("%s rect %v is (partly) off-screen %v", name, r, screen)
		}
	}
	for i, row := range m.presetRows {
		check("presetRow "+itoa(i), row.rect)
	}
	check("modeBtn", m.modeBtn)
	check("startCustom", m.startCustom)
	check("stepW-", m.stepW[0])
	check("stepW+", m.stepW[1])
	check("stepWin+", m.stepWin[1])
}
