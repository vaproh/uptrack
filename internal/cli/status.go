package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vaproh/uptrack/internal/asset"
	"github.com/vaproh/uptrack/internal/color"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
	"github.com/vaproh/uptrack/internal/platform"
	"github.com/vaproh/uptrack/internal/prompt"
	"github.com/vaproh/uptrack/internal/version"
)

// appStatus is the refreshed view of one tracked app.
type appStatus struct {
	app     config.App
	release *gh.Release
	err     error
}

func fetchReleases(app *config.App) ([]gh.Release, error) {
	owner, repo, err := gh.ParseRepo(app.Repo)
	if err != nil {
		return nil, err
	}
	client := gh.New("", gh.TokenFromEnv())
	rels, err := client.ListReleases(context.Background(), owner, repo)
	if err != nil {
		return nil, err
	}
	return rels, nil
}

// refreshAll fetches every app, records LatestVersion in state, and
// returns per-app results. One app failing does not stop the others.
func refreshAll(c *ctx) []appStatus {
	out := make([]appStatus, 0, len(c.cfg.Apps))
	for _, a := range c.cfg.Apps {
		st := appStatus{app: a}
		rels, err := fetchReleases(&a)
		if err != nil {
			st.err = err
			out = append(out, st)
			continue
		}
		if rel := gh.LatestRelease(rels, a.Channel); rel != nil {
			st.release = rel
			s := c.state.Apps[a.Name]
			s.LatestVersion = rel.TagName
			c.state.Apps[a.Name] = s
		}
		out = append(out, st)
	}
	_ = config.SaveState(c.paths.StateFile, c.state)
	return out
}

// selectForApp picks the install asset for a release. A stored rule
// that matches nothing falls back to automatic selection with a
// warning, so renamed upstream assets self-heal. Ties prompt on a
// terminal and fail otherwise.
func selectForApp(c *ctx, app *config.App, rel *gh.Release) (asset.Selection, error) {
	h := platform.Detect()
	sel, _, err := asset.Select(rel.Assets, app.Asset, h)
	if err != nil {
		var amb *asset.AmbiguousError
		if app.Asset != nil && !errors.As(err, &amb) {
			fmt.Printf("%s stored rule matched nothing, retrying automatically\n", color.Yellow("warning:"))
			sel, _, err = asset.Select(rel.Assets, nil, h)
		}
	}
	if err != nil {
		var amb *asset.AmbiguousError
		if errors.As(err, &amb) && prompt.IsTTY() && !c.assume {
			names := make([]string, len(amb.Candidates))
			for i, a := range amb.Candidates {
				names[i] = a.Name
			}
			idx, perr := prompt.Choose("multiple assets match; pick one:", names)
			if perr != nil {
				return asset.Selection{}, perr
			}
			return selectionOf(amb.Candidates[idx])
		}
		return asset.Selection{}, err
	}
	return sel, nil
}

// selectionOf rebuilds a Selection for an explicitly chosen asset.
func selectionOf(a gh.Asset) (asset.Selection, error) {
	kind, comp, ok := asset.KindOf(a.Name)
	if !ok {
		return asset.Selection{}, fmt.Errorf("cannot install %q: unknown format", a.Name)
	}
	return asset.Selection{Asset: a, Kind: kind, Compression: comp}, nil
}

// updateAvailable reports whether rel is newer than installed.
func updateAvailable(installed string, rel *gh.Release) bool {
	if rel == nil || installed == "" {
		return false
	}
	return version.Compare(installed, rel.TagName) < 0
}

// needsAttention reports whether an app should appear in outdated:
// not installed yet, or a newer release exists.
func needsAttention(installed string, rel *gh.Release) bool {
	if rel == nil {
		return false
	}
	return installed == "" || updateAvailable(installed, rel)
}

func newCheck() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Refresh release info for all tracked apps",
		RunE: runE(func(c *ctx, args []string) error {
			results := refreshAll(c)
			failed := 0
			for _, r := range results {
				printStatus(c, r)
				if r.err != nil {
					failed++
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d apps failed to refresh", failed, len(results))
			}
			return nil
		}),
	}
}

func newOutdated() *cobra.Command {
	return &cobra.Command{
		Use:   "outdated",
		Short: "Show apps needing attention (missing or upgradable)",
		RunE: runE(func(c *ctx, args []string) error {
			results := refreshAll(c)
			any := false
			for _, r := range results {
				if r.err != nil || r.release == nil {
					continue
				}
				if needsAttention(c.state.Apps[r.app.Name].InstalledVersion, r.release) {
					printStatus(c, r)
					any = true
				}
			}
			if !any {
				fmt.Println(color.Dim("everything is up to date"))
			}
			return nil
		}),
	}
}

func printStatus(c *ctx, r appStatus) {
	head := fmt.Sprintf("%s  %s", color.Bold(r.app.Name), color.Dim(r.app.Repo))
	if r.err != nil {
		fmt.Printf("%s\n  %s\n", head, color.Red("failed: "+r.err.Error()))
		return
	}
	if r.release == nil {
		fmt.Printf("%s\n  %s\n", head, color.Yellow("no release found for channel "+r.app.Channel))
		return
	}
	installed := c.state.Apps[r.app.Name].InstalledVersion
	rel := r.release
	tag := rel.TagName
	if rel.Prerelease {
		tag += color.Yellow(" (prerelease)")
	}
	switch {
	case installed == "":
		fmt.Printf("%s\n  latest %s  %s\n", head, color.Cyan(tag), color.Dim("not installed"))
	case updateAvailable(installed, rel):
		fmt.Printf("%s\n  installed %s  →  latest %s\n",
			head, color.Cyan(installed), color.Green(tag))
	default:
		fmt.Printf("%s\n  %s %s\n", head, color.Dim("up to date at"), color.Cyan(installed))
	}
}
