package ink

// Network stubs mirroring the real SDK's network helpers, so apps that bring
// the device's connection up (spelbutiken, and any future networked app)
// compile and play-test under the emulator. The test process already has the
// host's connectivity — net/http just works — so these succeed as no-ops.

// ConnectDefault matches the SDK call that auto-connects the default network
// (popping the system Wi-Fi dialog on hardware when needed).
func ConnectDefault() error { return nil }

// Connect connects a named network interface.
func Connect(name string) error { return nil }

// Disconnect drops the connection.
func Disconnect() error { return nil }

// KeepNetwork connects the default interface and returns a disconnect func.
func KeepNetwork() (func(), error) { return func() {}, nil }

// QueryNetwork matches the SDK's connection-state query (no-op here).
func QueryNetwork() {}

// OpenNetworkInfo shows the system network dialog on hardware (no-op here).
func OpenNetworkInfo() {}

// InitCerts pre-warms the system TLS certificate pool on hardware; the host's
// pool needs no warming.
func InitCerts() error { return nil }

// HwAddress returns the device MAC address.
func HwAddress() string { return "00:00:00:00:00:00" }

// Connections lists available network connections.
func Connections() []string { return nil }

// WirelessNetworks lists visible Wi-Fi networks.
func WirelessNetworks() []string { return nil }
