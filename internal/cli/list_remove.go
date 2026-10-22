package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/color"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/install"
	"github.com/vaproh/uptrack/internal/prompt"
)

func newList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracked applications",
		RunE: runE(func(c *ctx, args []string) error {
			if len(c.cfg.Apps) == 0 {
				fmt.Println(color.Dim("no tracked apps; see `uptrack add --help`"))
				return nil
			}
			for _, a := range c.cfg.Apps {
				st := c.state.Apps[a.Name]
				installed := st.InstalledVersion
				if installed == "" {
					installed = color.Dim("not installed")
				} else {
					installed = color.Cyan(installed)
				}
				latest := st.LatestVersion
				if latest == "" {
					latest = color.Dim("unknown")
				} else {
					latest = color.Cyan(latest)
				}
				fmt.Printf("%s  %s\n", color.Bold(a.Name), color.Dim(a.Repo))
				fmt.Printf("  %-10s %s\n", color.Dim("channel"), a.Channel)
				fmt.Printf("  %-10s %s\n", color.Dim("installed"), installed)
				fmt.Printf("  %-10s %s\n", color.Dim("latest"), latest)
			}
			return nil
		}),
	}
}

func newRemove() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Stop tracking an application (files are left alone)",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			return untrack(c, args[0])
		}),
	}
}

func newUninstall() *cobra.Command {
	var keepFiles, keepLaunchers, keepTracking bool
	cmd := &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Delete an application's files and stop tracking it",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			return uninstallFlow(c, args[0], keepFiles, keepLaunchers, keepTracking)
		}),
	}
	cmd.Flags().BoolVar(&keepFiles, "keep-files", false, "keep installed app files")
	cmd.Flags().BoolVar(&keepLaunchers, "keep-launchers", false, "keep launcher files")
	cmd.Flags().BoolVar(&keepTracking, "keep-tracking", false, "keep tracking the app")
	return cmd
}

// uninstallFlow removes the tracking entry, app files and launcher
// files. Interactively it shows what exists and asks per component;
// -y or the --keep flags decide without asking. Launcher files are
// the .desktop entries that put the app in your app menu.
func uninstallFlow(c *ctx, name string, keepFiles, keepLaunchers, keepTracking bool) error {
	app, err := c.cfg.Find(name)
	if err != nil {
		return err
	}
	st := c.state.Apps[name]
	dest := installDest(c, app)
	_, derr := os.Lstat(dest)
	hasFiles := derr == nil
	var launchers []string
	for _, f := range st.DesktopFiles {
		if _, err := os.Lstat(f); err == nil {
			launchers = append(launchers, f)
		}
	}

	doUntrack, doFiles, doLaunch := !keepTracking, hasFiles && !keepFiles, len(launchers) > 0 && !keepLaunchers
	if !c.assume {
		fmt.Printf("uninstall %s:\n", color.Bold(name))
		if doUntrack {
			fmt.Println("  tracking entry in config")
		}
		if doFiles {
			fmt.Printf("  app files at %s\n", color.Dim(dest))
		}
		if doLaunch {
			fmt.Println("  launcher files (.desktop entries for your app menu):")
			for _, f := range launchers {
				fmt.Printf("    %s\n", color.Dim(f))
			}
		}
		if !doUntrack && !doFiles && !doLaunch {
			fmt.Println(color.Dim("nothing to remove"))
			return nil
		}
		if doUntrack && !prompt.Confirm("stop tracking it (remove from config)?") {
			doUntrack = false
		}
		if doFiles && !prompt.Confirm(fmt.Sprintf("delete app files at %s?", dest)) {
			doFiles = false
		}
		if doLaunch && !prompt.Confirm("delete launcher files?") {
			doLaunch = false
		}
		if !doUntrack && !doFiles && !doLaunch {
			fmt.Println(color.Dim("aborted"))
			return nil
		}
	}

	if doLaunch {
		err := install.RemoveDesktopFiles(launchers)
		if err != nil {
			return err
		}
	}
	if doFiles {
		err = os.RemoveAll(dest)
		if err != nil {
			return fmt.Errorf("remove %s: %w", dest, err)
		}
		link := binLink(c, name)
		if _, err := os.Lstat(link); err == nil {
			err = os.Remove(link)
			if err != nil {
				return fmt.Errorf("remove %s: %w", link, err)
			}
		}
	}
	if doUntrack {
		_, err = dropApp(c, name)
		if err != nil {
			return err
		}
	} else {
		// Tracking stays, so state must match what remains.
		if doFiles {
			st.InstalledVersion = ""
			st.InstalledAsset = ""
			st.ExeRel = ""
			st.Verified = false
		}
		if doLaunch {
			st.DesktopFiles = nil
		}
		if doFiles || doLaunch {
			c.state.Apps[name] = st
			err = config.SaveState(c.paths.StateFile, c.state)
			if err != nil {
				return err
			}
		}
	}
	fmt.Printf("uninstalled %s\n", color.Bold(name))
	return nil
}

// untrack removes the config entry and state, leaving files alone.
func untrack(c *ctx, name string) error {
	_, err := dropApp(c, name)
	if err != nil {
		return err
	}
	fmt.Printf("removed %s (files left in place)\n", color.Bold(name))
	return nil
}

// dropApp removes the config entry and state, returning the app.
func dropApp(c *ctx, name string) (*config.App, error) {
	kept := c.cfg.Apps[:0]
	var app *config.App
	for i := range c.cfg.Apps {
		if c.cfg.Apps[i].Name == name {
			app = &c.cfg.Apps[i]
		} else {
			kept = append(kept, c.cfg.Apps[i])
		}
	}
	if app == nil {
		return nil, fmt.Errorf("%w: %s", config.ErrNotFound, name)
	}
	c.cfg.Apps = kept
	err := config.SaveConfig(c.paths.ConfigFile, c.cfg)
	if err != nil {
		return nil, err
	}
	delete(c.state.Apps, name)
	err = config.SaveState(c.paths.StateFile, c.state)
	if err != nil {
		return nil, err
	}
	return app, nil
}

func binLink(c *ctx, name string) string {
	return c.paths.BinDir + string(os.PathSeparator) + name
}

func installDest(c *ctx, a *config.App) string {
	if a.Install != nil && a.Install.Dest != "" {
		return expandHome(a.Install.Dest)
	}
	return c.paths.OptDir + string(os.PathSeparator) + a.Name
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + p[1:]
		}
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(p, "$HOME/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + p[5:]
		}
	}
	return os.ExpandEnv(p)
}
