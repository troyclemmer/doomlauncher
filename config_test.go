package main

import (
	"os"
	"path/filepath"
	"testing"
)

// This test writes a config, so it must never touch a real Doom folder. An
// earlier version used the live directory and deleted the user's launcher.json.
func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := configPath(dir)
	if filepath.Dir(p) != dir {
		t.Fatalf("configPath escaped the temp dir: %s", p)
	}

	c := defaultConfig()
	c.LastCampaign = "Sunder 2308"
	c.BrutalOn = boolPtr(false)
	c.CampaignIWAD["deathless"] = "DOOM.WAD"
	c.WadStatus["sunlust"] = StatusProgress
	if err := c.save(p); err != nil {
		t.Fatal(err)
	}

	back := loadConfig(p)
	switch {
	case back.LastCampaign != "Sunder 2308":
		t.Errorf("LastCampaign = %q", back.LastCampaign)
	case *back.BrutalOn != false:
		t.Errorf("BrutalOn = %v", *back.BrutalOn)
	case back.CampaignIWAD["deathless"] != "DOOM.WAD":
		t.Errorf("CampaignIWAD = %v", back.CampaignIWAD)
	case back.WadStatus["sunlust"] != StatusProgress:
		t.Errorf("WadStatus = %v", back.WadStatus)
	case len(back.Presets) != 1:
		t.Errorf("Presets = %v", back.Presets)
	}
}

// A config written by an older build has no wad_status key at all. Loading it
// must not leave a nil map for the status code to write into.
func TestLoadConfigBackfillsMaps(t *testing.T) {
	p := filepath.Join(t.TempDir(), "launcher.json")
	if err := os.WriteFile(p, []byte(`{"engine":"uzdoom.exe"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := loadConfig(p)
	if c.WadStatus == nil || c.CampaignIWAD == nil {
		t.Fatal("loadConfig left a nil map; status writes would panic")
	}
	c.WadStatus["x"] = StatusDone // would panic on a nil map
}
