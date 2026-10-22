package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"uptrack/internal/asset"
	"uptrack/internal/color"
	"uptrack/internal/config"
	"uptrack/internal/gh"
	"uptrack/internal/prompt"
)

func newAdd() *cobra.Command {
	var name, channel, match, types, exe, dest string
	cmd := &cobra.Command{
		Use:   "add <owner/repo|url>",
		Short: "Track a GitHub repository",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			owner, repo, err := gh.ParseRepo(args[0])
			if err != nil {
				return err
			}
			if name == "" {
				name = repo
			}
			if _, err := c.cfg.Find(name); err == nil {
				return fmt.Errorf("%s is already tracked", name)
			}
			switch channel {
			case "stable", "prerelease", "any":
			default:
				return fmt.Errorf("channel must be stable, prerelease or any")
			}
			app := config.App{Name: name, Repo: owner + "/" + repo, Channel: channel}
			if match != "" || types != "" || exe != "" || dest != "" {
				app.Asset = &config.AssetRule{Match: match}
				app.Install = &config.InstallSpec{Type: types, Exe: exe, Dest: dest}
				if app.Install.Type == "" && app.Install.Exe == "" && app.Install.Dest == "" {
					app.Install = nil
				}
				if app.Asset.Match == "" {
					app.Asset = nil
				}
			}

			rels, err := fetchReleases(&app)
			if err != nil {
				return err
			}
			rel := gh.LatestRelease(rels, channel)
			if rel == nil {
				return fmt.Errorf("no release found for channel %q", channel)
			}
			sel, _, serr := asset.Select(rel.Assets, app.Asset, hostOf())
			if serr != nil {
				var amb *asset.AmbiguousError
				if errors.As(serr, &amb) && prompt.IsTTY() {
					names := make([]string, len(amb.Candidates))
					for i, a := range amb.Candidates {
						names[i] = a.Name
					}
					idx, perr := prompt.Choose("multiple assets match; pick the one to pin:", names)
					if perr != nil {
						return perr
					}
					// Pin the exact choice; a later rename self-heals
					// through the install-time fallback.
					app.Asset = &config.AssetRule{Match: amb.Candidates[idx].Name}
					sel, _, serr = asset.Select(rel.Assets, app.Asset, hostOf())
				}
				if serr != nil {
					return serr
				}
			}

			c.cfg.Apps = append(c.cfg.Apps, app)
			err = config.SaveConfig(c.paths.ConfigFile, c.cfg)
			if err != nil {
				return err
			}
			s := c.state.Apps[name]
			s.LatestVersion = rel.TagName
			c.state.Apps[name] = s
			err = config.SaveState(c.paths.StateFile, c.state)
			if err != nil {
				return err
			}
			fmt.Printf("tracking %s  %s\n", color.Bold(name), color.Dim(app.Repo))
			fmt.Printf("  latest %s, asset %s\n", color.Cyan(rel.TagName), color.Cyan(sel.Asset.Name))
			fmt.Printf("run %s to install it\n", color.Dim("uptrack install "+name))
			return nil
		}),
	}
	cmd.Flags().StringVar(&name, "name", "", "local name (default: repo name)")
	cmd.Flags().StringVar(&channel, "channel", "stable", "release channel: stable|prerelease|any")
	cmd.Flags().StringVar(&match, "asset", "", "asset glob override")
	cmd.Flags().StringVar(&types, "type", "", "install type override: binary|archive|appimage|deb")
	cmd.Flags().StringVar(&exe, "exe", "", "executable name override")
	cmd.Flags().StringVar(&dest, "dest", "", "install destination override")
	return cmd
}

func newInfo() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show latest release notes and the selected asset",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			app, err := c.cfg.Find(args[0])
			if err != nil {
				return err
			}
			rels, err := fetchReleases(app)
			if err != nil {
				return err
			}
			rel := gh.LatestRelease(rels, app.Channel)
			if rel == nil {
				return fmt.Errorf("no release found for channel %q", app.Channel)
			}
			fmt.Printf("%s  %s\n", color.Bold(app.Name), color.Dim(app.Repo))
			ch := app.Channel
			if ch != "stable" {
				ch = color.Yellow(ch)
			}
			fmt.Printf("channel %s  latest %s", ch, color.Cyan(rel.TagName))
			if rel.Prerelease {
				fmt.Printf(" %s", color.Yellow("(prerelease)"))
			}
			fmt.Println()
			if !rel.PublishedAt.IsZero() {
				fmt.Printf("published %s\n", rel.PublishedAt.Format("2006-01-02"))
			}
			sel, log, err := asset.Select(rel.Assets, app.Asset, hostOf())
			if err != nil {
				fmt.Printf("asset: %s\n", color.Red(err.Error()))
			} else {
				fmt.Printf("asset %s  (%s)\n", color.Cyan(sel.Asset.Name), sel.Kind)
			}
			fmt.Println(color.Bold("candidates:"))
			for _, l := range log {
				mark := color.Dim("-")
				switch l.Decision {
				case "keep":
					mark = color.Green("+")
				case "demote":
					mark = color.Yellow("~")
				case "reject":
					mark = color.Red("x")
				}
				fmt.Printf("  %s %-45s %s\n", mark, l.Asset, color.Dim(l.Reason))
			}
			if body := strings.TrimSpace(rel.Body); body != "" {
				fmt.Println(color.Bold("notes:"))
				lines := strings.Split(body, "\n")
				if len(lines) > 40 {
					lines = append(lines[:40], "...")
				}
				for _, l := range lines {
					fmt.Printf("  %s\n", l)
				}
			}
			return nil
		}),
	}
}

func newLog() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "log <name>",
		Short: "Show release history",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			app, err := c.cfg.Find(args[0])
			if err != nil {
				return err
			}
			rels, err := fetchReleases(app)
			if err != nil {
				return err
			}
			n := 0
			for _, rel := range rels {
				if n >= limit {
					break
				}
				if app.Channel == "stable" && rel.Prerelease {
					continue
				}
				date := ""
				if !rel.PublishedAt.IsZero() {
					date = rel.PublishedAt.Format("2006-01-02")
				}
				pre := ""
				if rel.Prerelease {
					pre = " " + color.Yellow("(prerelease)")
				}
				fmt.Printf("%s  %s%s\n", color.Cyan(rel.TagName), color.Dim(date), pre)
				n++
			}
			return nil
		}),
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum releases to show")
	return cmd
}
