package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/color"
	"github.com/vaproh/uptrack/internal/install"
	"github.com/vaproh/uptrack/internal/prompt"
)

func newUninstallSelf() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "uninstall-self",
		Short: "Remove uptrack itself (config, state and binary)",
		Args:  cobra.NoArgs,
		RunE: runE(func(c *ctx, args []string) error {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate own binary: %w", err)
			}
			exe, err = filepath.EvalSymlinks(exe)
			if err != nil {
				return fmt.Errorf("resolve own binary: %w", err)
			}
			if !c.assume {
				fmt.Println("this removes:")
				fmt.Printf("  uptrack binary %s\n", exe)
				fmt.Printf("  config %s\n", c.paths.ConfigFile)
				fmt.Printf("  state %s\n", c.paths.StateFile)
				if all {
					for _, a := range c.cfg.Apps {
						fmt.Printf("  app files %s\n", installDest(c, &a))
					}
				} else {
					fmt.Println("  (installed apps are kept; --all removes them too)")
				}
				if !prompt.Confirm("uninstall uptrack?") {
					fmt.Println(color.Dim("aborted"))
					return nil
				}
			}
			removed, err := uninstallSelf(c, exe, all)
			if err != nil {
				return err
			}
			for _, r := range removed {
				fmt.Printf("removed %s\n", color.Dim(r))
			}
			fmt.Println("uptrack is gone. to reinstall: curl -fsSL https://raw.githubusercontent.com/vaproh/uptrack/main/install.sh | sh")
			return nil
		}),
	}
	cmd.Flags().BoolVar(&all, "all", false, "also delete installed app files and symlinks")
	return cmd
}

// uninstallSelf deletes launcher files, optionally app files, then
// config, state and the binary itself, in that order. The binary goes
// last; removing a running executable is safe on Linux. Pure of
// prompts so tests can call it.
func uninstallSelf(c *ctx, exe string, removeApps bool) ([]string, error) {
	var removed []string
	for _, files := range stateDesktopFiles(c) {
		err := install.RemoveDesktopFiles(files)
		if err != nil {
			return removed, err
		}
		removed = append(removed, files...)
	}
	if removeApps {
		for i := range c.cfg.Apps {
			app := &c.cfg.Apps[i]
			err := os.RemoveAll(installDest(c, app))
			if err != nil {
				return removed, err
			}
			removed = append(removed, installDest(c, app))
			link := binLink(c, app.Name)
			if _, err := os.Lstat(link); err == nil {
				err = os.Remove(link)
				if err != nil {
					return removed, err
				}
				removed = append(removed, link)
			}
		}
	}
	for _, p := range []string{c.paths.ConfigFile, c.paths.StateFile} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed = append(removed, p)
		removeEmptyParents(p)
	}
	err := os.Remove(exe)
	if err != nil {
		return removed, fmt.Errorf("remove binary %s: %w", exe, err)
	}
	removed = append(removed, exe)
	return removed, nil
}

func stateDesktopFiles(c *ctx) [][]string {
	var out [][]string
	for _, s := range c.state.Apps {
		if len(s.DesktopFiles) > 0 {
			out = append(out, s.DesktopFiles)
		}
	}
	return out
}

// removeEmptyParents removes empty parent dirs up to (not including)
// the home directory. Best effort; stops at the first non-empty dir.
func removeEmptyParents(p string) {
	home, _ := os.UserHomeDir()
	for dir := filepath.Dir(p); ; dir = filepath.Dir(dir) {
		if dir == home || dir == "/" || dir == "." || dir == "" {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}
