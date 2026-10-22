package cli

import (
	"fmt"
	"strings"

	"github.com/vaproh/uptrack/internal/color"
)

// downloadBar returns a progress callback rendering a single-line bar.
// It returns nil when output is not a terminal, keeping installs quiet
// in scripts and pipes.
func downloadBar() func(downloaded, total int64) {
	if !color.Enabled() {
		return nil
	}
	return func(done, total int64) {
		if total > 0 {
			pct := float64(done) / float64(total) * 100
			if pct > 100 {
				pct = 100
			}
			const width = 20
			filled := int(pct/100*width + 0.5)
			// \x1b[K clears to end of line so a shorter redraw never
			// leaves ghost characters from a longer one.
			fmt.Printf("\r  [%s%s] %5.1f%%  %s / %s\x1b[K",
				strings.Repeat("█", filled), strings.Repeat("░", width-filled),
				pct, humanSize(done), humanSize(total))
		} else {
			fmt.Printf("\r  %s downloaded\x1b[K", humanSize(done))
		}
	}
}

// finishBar ends the progress line.
func finishBar() {
	if color.Enabled() {
		fmt.Println()
	}
}
