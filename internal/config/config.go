// Package config owns the user-editable YAML config and the
// machine-local JSON state. Config is for dotfiles; state is not.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var ErrNotFound = errors.New("app not found")

// AssetRule optionally overrides automatic asset selection.
type AssetRule struct {
	Match   string   `yaml:"match,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// InstallSpec optionally overrides install inference.
type InstallSpec struct {
	Type string `yaml:"type,omitempty"` // appimage | binary | archive | deb
	Dest string `yaml:"dest,omitempty"` // default ~/.local/opt/<name>
	Exe  string `yaml:"exe,omitempty"`  // binary name or path inside archive
}

// App is one tracked application. Only Name, Repo and Channel are required.
type App struct {
	Name    string       `yaml:"name"`
	Repo    string       `yaml:"repo"`
	Channel string       `yaml:"channel"`
	Asset   *AssetRule   `yaml:"asset,omitempty"`
	Install *InstallSpec `yaml:"install,omitempty"`
}

// Config is the whole config.yaml.
type Config struct {
	Apps []App `yaml:"apps"`
}

func (c *Config) Find(name string) (*App, error) {
	for i := range c.Apps {
		if c.Apps[i].Name == name {
			return &c.Apps[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
}

// AppState is per-app machine-local state.
type AppState struct {
	InstalledVersion string `json:"installedVersion,omitempty"`
	InstalledAsset   string `json:"installedAsset,omitempty"`
	Verified         bool   `json:"verified,omitempty"`
	LatestVersion    string `json:"latestVersion,omitempty"`
}

// State is the whole state.json.
type State struct {
	Apps map[string]AppState `json:"apps"`
}

// Paths resolves config and state file locations.
type Paths struct {
	ConfigFile string
	StateFile  string
	BinDir     string
	OptDir     string
}

// DefaultPaths honors XDG_CONFIG_HOME for config and fixed
// ~/.local locations for binaries, apps and state.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("home dir: %w", err)
	}
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("config dir: %w", err)
	}
	local := filepath.Join(home, ".local")
	return Paths{
		ConfigFile: filepath.Join(cfgDir, "uptrack", "config.yaml"),
		StateFile:  filepath.Join(local, "state", "uptrack", "state.json"),
		BinDir:     filepath.Join(local, "bin"),
		OptDir:     filepath.Join(local, "opt"),
	}, nil
}

// LoadConfig reads config.yaml. A missing file yields an empty config.
func LoadConfig(path string) (*Config, error) {
	c := &Config{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	err = yaml.Unmarshal(raw, c)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return c, nil
}

// SaveConfig writes config.yaml atomically.
func SaveConfig(path string, c *Config) error {
	raw, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	err = writeAtomic(path, raw, 0o644)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, err = tmp.Write(data)
	if err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	err = tmp.Chmod(perm)
	if err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	err = tmp.Close()
	if err != nil {
		os.Remove(tmpName)
		return err
	}
	err = os.Rename(tmpName, path)
	if err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
