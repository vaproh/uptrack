package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"uptrack/internal/color"
	"uptrack/internal/config"
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
	var purge bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Stop tracking an application (files are left alone)",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			return removeApp(c, args[0], purge)
		}),
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete installed files and symlink")
	return cmd
}

func removeApp(c *ctx, name string, purge bool) error {
	kept := c.cfg.Apps[:0]
	found := false
	var app config.App
	for _, a := range c.cfg.Apps {
		if a.Name == name {
			found = true
			app = a
		} else {
			kept = append(kept, a)
		}
	}
	if !found {
		return fmt.Errorf("%w: %s", config.ErrNotFound, name)
	}
	c.cfg.Apps = kept
	err := config.SaveConfig(c.paths.ConfigFile, c.cfg)
	if err != nil {
		return err
	}
	delete(c.state.Apps, name)
	err = config.SaveState(c.paths.StateFile, c.state)
	if err != nil {
		return err
	}
	if purge {
		dest := installDest(c, &app)
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
		fmt.Printf("removed %s and its files\n", color.Bold(name))
	} else {
		fmt.Printf("removed %s (files left in place)\n", color.Bold(name))
	}
	return nil
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
