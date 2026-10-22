package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/color"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
	"github.com/vaproh/uptrack/internal/install"
	"github.com/vaproh/uptrack/internal/prompt"
)

func newInstall() *cobra.Command {
	var tag string
	cmd := &cobra.Command{
		Use:   "install <name>",
		Short: "Install or upgrade one tracked app",
		Args:  cobra.ExactArgs(1),
		RunE: runE(func(c *ctx, args []string) error {
			app, err := c.cfg.Find(args[0])
			if err != nil {
				return err
			}
			return installApp(c, app, tag)
		}),
	}
	cmd.Flags().StringVar(&tag, "version", "", "install a specific release tag instead of the latest")
	return cmd
}

func newSync() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Install missing apps and upgrade outdated ones",
		RunE: runE(func(c *ctx, args []string) error {
			type job struct {
				app config.App
				tag string
			}
			var jobs []job
			results := refreshAll(c)
			var skipped []string
			for _, r := range results {
				if r.err != nil || r.release == nil {
					printStatus(c, r)
					continue
				}
				installed := c.state.Apps[r.app.Name].InstalledVersion
				tag := syncTag(r.app, installed, r.release)
				if tag == "" {
					if r.app.Pin != "" {
						skipped = append(skipped, r.app.Name)
					}
					continue
				}
				jobs = append(jobs, job{r.app, tag})
			}
			if len(jobs) == 0 {
				if len(skipped) == 0 {
					fmt.Println(color.Dim("everything is up to date"))
				} else {
					fmt.Printf("%s pinned, nothing to do: %s\n",
						color.Dim("skipped"), strings.Join(skipped, ", "))
				}
				return nil
			}
			if !c.assume {
				fmt.Println("plan:")
				for _, j := range jobs {
					installed := c.state.Apps[j.app.Name].InstalledVersion
					if installed == "" {
						fmt.Printf("  install %s %s\n", j.app.Name, color.Cyan(j.tag))
					} else {
						fmt.Printf("  upgrade %s %s → %s\n", j.app.Name, color.Cyan(installed), color.Green(j.tag))
					}
				}
				if !prompt.Confirm("proceed?") {
					fmt.Println(color.Dim("aborted"))
					return nil
				}
			}
			failed := 0
			for _, j := range jobs {
				app, err := c.cfg.Find(j.app.Name)
				if err != nil {
					fmt.Printf("%s: %s\n", j.app.Name, color.Red(err.Error()))
					failed++
					continue
				}
				err = installApp(c, app, j.tag)
				if err != nil {
					failed++
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d installs failed", failed, len(jobs))
			}
			return nil
		}),
	}
	return cmd
}

// installApp resolves the release, confirms, installs and records state.
// A bare install of a pinned app installs the pin; --version overrides.
func installApp(c *ctx, app *config.App, tag string) error {
	if tag == "" && app.Pin != "" {
		tag = app.Pin
	}
	rels, err := fetchReleases(app)
	if err != nil {
		return err
	}
	var rel *gh.Release
	if tag != "" {
		rel = gh.FindRelease(rels, tag)
		if rel == nil {
			return fmt.Errorf("release %q not found", tag)
		}
	} else {
		rel = gh.LatestRelease(rels, app.Channel)
		if rel == nil {
			return fmt.Errorf("no release found for channel %q", app.Channel)
		}
	}
	sel, err := selectForApp(c, app, rel)
	if err != nil {
		return err
	}
	dest := installDest(c, app)
	if !c.assume {
		size := ""
		if sel.Asset.Size > 0 {
			size = " (" + humanSize(sel.Asset.Size) + ")"
		}
		if !prompt.Confirm(fmt.Sprintf("install %s %s%s to %s?", app.Name, rel.TagName, size, dest)) {
			fmt.Println(color.Dim("aborted"))
			return nil
		}
	}
	res, err := install.Install(context.Background(), install.Request{
		App:      *app,
		Sel:      sel,
		Tag:      rel.TagName,
		DestDir:  dest,
		BinDir:   c.paths.BinDir,
		Progress: downloadBar(),
	})
	finishBar()
	if err != nil {
		return err
	}
	s := c.state.Apps[app.Name]
	s.InstalledVersion = rel.TagName
	s.InstalledAsset = sel.Asset.Name
	s.ExeRel = res.ExeRel
	s.Verified = res.Verified
	s.LatestVersion = rel.TagName
	c.state.Apps[app.Name] = s
	err = config.SaveState(c.paths.StateFile, c.state)
	if err != nil {
		return err
	}
	if !res.Verified {
		fmt.Printf("%s\n", color.Yellow("unverified: upstream published no digest for this asset"))
	}
	if !install.InPath(c.paths.BinDir) {
		fmt.Printf("%s %s is not on PATH; add this to your shell config:\n", color.Yellow("warning:"), c.paths.BinDir)
		fmt.Printf("  export PATH=\"$PATH:%s\"\n", c.paths.BinDir)
	}
	fmt.Printf("installed %s %s → %s\n", color.Bold(app.Name), color.Cyan(rel.TagName), color.Dim(dest))
	return nil
}

// humanSize formats bytes for the confirm prompt.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	suffix := "KMGT"[exp : exp+1]
	return fmt.Sprintf("%.1f %sB", float64(n)/float64(div), suffix)
}
