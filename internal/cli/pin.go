package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
)

// saveConfig persists the tracked-app list.
func saveConfig(c *ctx) error {
	return config.SaveConfig(c.paths.ConfigFile, c.cfg)
}

// pin holds an app at one version: sync skips it until unpinned.
func newPin() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <name> [tag]",
		Short: "Hold an app at a version (default: installed version)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: runE(func(c *ctx, args []string) error {
			app, err := c.cfg.Find(args[0])
			if err != nil {
				return err
			}
			tag := ""
			if len(args) == 2 {
				tag = args[1]
			} else {
				tag = c.state.Apps[app.Name].InstalledVersion
				if tag == "" {
					tag = c.state.Apps[app.Name].LatestVersion
				}
				if tag == "" {
					return fmt.Errorf("nothing installed and no release known; give a tag")
				}
			}
			rels, err := fetchReleases(app)
			if err != nil {
				return err
			}
			if gh.FindRelease(rels, tag) == nil {
				return fmt.Errorf("release %q not found", tag)
			}
			app.Pin = tag
			err = saveConfig(c)
			if err != nil {
				return err
			}
			fmt.Printf("pinned %s at %s\n", app.Name, tag)
			return nil
		}),
	}
}

func newUnpin() *cobra.Command {
	return &cobra.Command{
		Use:   "unpin <name>",
		Short: "Release a pin; sync follows the channel again",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			app, err := c.cfg.Find(args[0])
			if err != nil {
				return err
			}
			if app.Pin == "" {
				fmt.Printf("%s is not pinned\n", app.Name)
				return nil
			}
			app.Pin = ""
			err = saveConfig(c)
			if err != nil {
				return err
			}
			fmt.Printf("unpinned %s\n", app.Name)
			return nil
		}),
	}
}
