package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/color"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/install"
)

func newDoctor() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check installs for problems",
		RunE: runE(func(c *ctx, args []string) error {
			issues := 0
			if !install.InPath(c.paths.BinDir) {
				fmt.Printf("%s %s is not on PATH\n", color.Yellow("warn:"), c.paths.BinDir)
			}
			for _, a := range c.cfg.Apps {
				for _, msg := range checkApp(c, &a) {
					fmt.Printf("%s: %s\n", color.Bold(a.Name), color.Red(msg))
					issues++
				}
			}
			if issues == 0 {
				fmt.Println(color.Green("all installs healthy"))
				return nil
			}
			return fmt.Errorf("%d problem%s found", issues, plural(issues))
		}),
	}
}

// checkApp verifies one install: destination, executable, symlink and
// shared libraries. Pure of printing so tests can call it.
func checkApp(c *ctx, app *config.App) []string {
	var out []string
	st := c.state.Apps[app.Name]
	if st.InstalledVersion == "" {
		return nil
	}
	dest := installDest(c, app)
	if fi, err := os.Stat(dest); err != nil || !fi.IsDir() {
		return []string{fmt.Sprintf("install dir missing: %s", dest)}
	}
	if st.ExeRel == "" {
		out = append(out, "exe untracked (predates exe tracking); reinstall to enable full checks")
		return out
	}
	exe := filepath.Join(dest, st.ExeRel)
	fi, err := os.Stat(exe)
	if err != nil {
		out = append(out, fmt.Sprintf("executable missing: %s", exe))
		return out
	}
	if fi.Mode().Perm()&0o111 == 0 {
		out = append(out, fmt.Sprintf("executable not runnable: %s", exe))
	}
	link := binLink(c, app.Name)
	target, err := os.Readlink(link)
	if err != nil {
		out = append(out, fmt.Sprintf("symlink missing: %s", link))
	} else if target != exe {
		out = append(out, fmt.Sprintf("symlink points at %s, want %s", target, exe))
	}
	for _, lib := range install.MissingLibs(exe) {
		out = append(out, fmt.Sprintf("missing shared library: %s", lib))
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
