package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preset is a named group of files loaded together with -file.
// Files are either bare names (relative to the game dir) or absolute paths.
type Preset struct {
	Name  string   `json:"name"`
	Files []string `json:"files"`
}

// Config is persisted next to the exe as launcher.json so the launcher
// comes back up exactly how it was left.
type Config struct {
	Engine       string            `json:"engine"`
	ModFile      string            `json:"mod_file"`
	Presets      []Preset          `json:"presets"`
	LastIWAD     string            `json:"last_iwad"`
	LastCampaign string            `json:"last_campaign"`
	LastSkill    int               `json:"last_skill"`
	ExtraArgs    string            `json:"extra_args"`
	BrutalOn     *bool             `json:"brutal_on"`
	CampaignIWAD map[string]string `json:"campaign_iwad"`
	WadStatus    map[string]string `json:"wad_status"`
}

func boolPtr(b bool) *bool { return &b }

func defaultConfig() *Config {
	return &Config{
		Engine:   "uzdoom.exe",
		BrutalOn: boolPtr(true),
		LastIWAD: "DOOM2.WAD", // the usual case; per-campaign choices are learned after that
		Presets: []Preset{
			{
				Name:  "Extermination Day",
				Files: []string{"EdayTest001.pk3", "EdayMusic001.wad", "EdayMapInfoPatchV2.wad"},
			},
		},
		CampaignIWAD: map[string]string{},
		WadStatus:    map[string]string{},
	}
}

// configPath prefers the game dir so the whole setup stays portable,
// and falls back to %APPDATA% when that dir turns out to be read-only.
func configPath(gameDir string) string {
	local := filepath.Join(gameDir, "launcher.json")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	probe := filepath.Join(gameDir, ".launcher-write-test")
	if f, err := os.Create(probe); err == nil {
		f.Close()
		os.Remove(probe)
		return local
	}
	dir := filepath.Join(os.Getenv("APPDATA"), "DoomLauncher")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "launcher.json")
}

func loadConfig(path string) *Config {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return defaultConfig()
	}
	if cfg.Engine == "" {
		cfg.Engine = "uzdoom.exe"
	}
	if cfg.BrutalOn == nil {
		cfg.BrutalOn = boolPtr(true)
	}
	if cfg.CampaignIWAD == nil {
		cfg.CampaignIWAD = map[string]string{}
	}
	if cfg.WadStatus == nil {
		cfg.WadStatus = map[string]string{}
	}
	return cfg
}

func (c *Config) save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
