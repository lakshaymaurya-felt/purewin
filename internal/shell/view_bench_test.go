package shell

import (
	"testing"
)

func BenchmarkShellView(b *testing.B) {
	m := NewShellModel("0.6.0")
	m.Width = 120
	m.Height = 40
	// Add some output lines
	for i := 0; i < 100; i++ {
		m.AppendOutput("wm ❯ /clean --dry-run")
		m.AppendOutput("  Scanning for cleanable files...")
		m.AppendOutput("  Found 42 items totaling 1.2 GiB")
		m.AppendOutput("")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkShellViewWithCompletions(b *testing.B) {
	m := NewShellModel("0.6.0")
	m.Width = 120
	m.Height = 40
	m.completions.Open()
	m.completions.Filter("")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkPadToWidth(b *testing.B) {
	s := "  🧹 /clean       Deep clean system caches"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = padToWidth(s, 52)
	}
}
