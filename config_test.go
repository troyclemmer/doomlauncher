package main

import (
	"fmt"
	"os"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := doomDir(t)
	p := configPath(dir)
	fmt.Println("resolved config path:", p)

	c := defaultConfig()
	c.LastCampaign = "Sunder 2308"
	c.BrutalOn = boolPtr(false)
	c.CampaignIWAD["deathless"] = "DOOM.WAD"
	if err := c.save(p); err != nil {
		t.Fatal(err)
	}
	back := loadConfig(p)
	if back.LastCampaign != "Sunder 2308" || *back.BrutalOn != false ||
		back.CampaignIWAD["deathless"] != "DOOM.WAD" || len(back.Presets) != 1 {
		t.Fatalf("round trip mismatch: %+v", back)
	}
	// leave the folder as we found it
	os.Remove(p)
	fmt.Println("round trip OK, removed test file")
}
