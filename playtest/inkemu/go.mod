// Module github.com/dennwc/inkview is a PURE-GO, cgo-free re-implementation of
// the subset of the dennwc/inkview SDK that the games in this repo use. It is a
// drop-in replacement (same import path) enabled per-game via a go.work overlay
// (see playtest/play.sh) so a game can be *played headlessly* on a normal PC:
// the app's real Init/Draw/Pointer/Key path runs against an in-memory
// framebuffer, and a Harness injects taps and key presses.
//
// This exists ONLY for the play-test harness. Device builds still use the cgo
// vendor under each game's third_party/inkview.
module github.com/dennwc/inkview

go 1.25.0

require golang.org/x/image v0.43.0

require golang.org/x/text v0.38.0 // indirect
