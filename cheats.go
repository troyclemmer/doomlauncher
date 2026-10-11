package main

import (
	"strings"

	"github.com/lxn/walk"
)

// cheatItem is one console command. Every command listed here was verified to
// exist in this engine build before being added.
type cheatItem struct {
	Category string
	Command  string
	Desc     string
}

// cheatItems is the reference shown in the Cheatsheet tab. Ordering is
// deliberate: the map-progression commands come first because getting stranded
// on the wrong map is the thing you actually need the console for.
var cheatItems = []cheatItem{
	{"Maps", "changemap MAP21", "Go to MAP21 as if you had walked through an exit - KEEPS weapons, ammo, armour and health."},
	{"Maps", "map MAP21", "Go to MAP21 as a brand new game. Does not keep inventory. Use changemap to keep inventory."},
	{"Maps", "nextmap", "Go to the next map in order. Does not keep inventory. Use changemap to keep inventory."},
	{"Maps", "nextsecret", "Go to the secret map. Does not keep inventory. Use changemap to keep inventory."},
	{"Maps", "restart", "Restart the current map from the beginning."},
	{"Maps", "open somemap.wad", "Load and play a map straight out of a wad file."},
	{"Maps", "warp 1024 -512", "Teleport to x/y coordinates inside the current map."},

	{"Items", "give all", "Give every weapon, ammo, armour, health and keys."},
	{"Items", "give weapons", "Give every weapon, without touching anything else."},
	{"Items", "give ammo", "Fill all ammo types to their CURRENT max. Run give backpack first or you only fill to the base caps."},
	{"Items", "give armor", "Give armour."},
	{"Items", "give keys", "Give every key in the map - use when a key is unreachable."},
	{"Items", "give health", "Restore health."},
	{"Items", "give backpack", "Give the backpack (doubles ammo capacity)."},
	{"Items", "take all", "Strip everything - handy for faking a pistol start."},
	{"Items", "printinv", "Print everything you are carrying to the console."},

	{"Weapons", "give shotgun", "Vanilla names: shotgun, supershotgun, chaingun, rocketlauncher, plasmarifle, bfg9000, chainsaw, fist. BD EXTRAS (Brutal Doom only, no vanilla equivalent): Rifle, Revolver, Sniper, RailGun, Unmaker, MG42, MP40, AssaultShotgun, BFG10k, Flamethrower2, FlameCannon, GrenadeLauncher, IncendiaryLauncher, HellishMissileLauncher, NukeLauncher, Devastator, HandGrenades, BrutalAxe, BrutalSMG, DualPistols, DualSMG, DualRifles, DualMP40, DualPlasmaRifles, DualFlameCannon."},
	{"Weapons", "give berserk", "Berserk pack - permanent for the map, big fist damage."},
	{"Weapons", "give clip 999", "Give a specific type and amount, clamped to the max: give clip 999, give shell 999, give rocketammo 999, give cell 999."},
	{"Weapons", "give backpack", "Doubles your max ammo capacity. It does NOT stack - once is enough, repeating does nothing at all."},
	{"Weapons", "give backpack; give ammo", "Max ammo in one line. Backpack FIRST to raise the caps, then fill them. A semicolon chains console commands."},
	{"Weapons", "dumpspawnables", "List every actor name currently loaded. THE way to find what to give when a mod is on."},
	{"Weapons", "give Shot_Gun", "Brutal Doom shotgun. BD replaces the vanilla pickups with its own classes, so the vanilla names may not give what you expect. Direct equivalents: Shot_Gun, SSG, MiniGun, Rocket_Launcher, Plasma_Gun, BIG_FUCKING_GUN, Chain_saw, BrutalPistol. See the give shotgun row for the BD-only extras."},
	{"Weapons", "give SSG", "Brutal Doom super shotgun."},
	{"Weapons", "give MiniGun", "Brutal Doom chaingun/minigun."},
	{"Weapons", "give Rocket_Launcher", "Brutal Doom rocket launcher."},
	{"Weapons", "give Plasma_Gun", "Brutal Doom plasma rifle."},
	{"Weapons", "give BIG_FUCKING_GUN", "Brutal Doom BFG. Yes, that is the actual class name."},
	{"Weapons", "give Chain_saw", "Brutal Doom chainsaw."},
	{"Weapons", "give Rifle", "Brutal Doom assault rifle - one of the BD-only weapons with no vanilla counterpart."},
	{"Weapons", "give Flamethrower2", "Brutal Doom flamethrower. Purist-mode variants of most BD weapons exist with a Purist prefix, e.g. PuristSSG."},

	{"Survival", "god", "Invulnerable. Run it again to switch off."},
	{"Survival", "buddha", "You take damage but never drop below 1 HP."},
	{"Survival", "resurrect", "Come back to life after dying, on the spot."},
	{"Survival", "notarget", "Monsters stop noticing you."},
	{"Survival", "noclip", "Walk through walls."},
	{"Survival", "noclip2", "Walk through walls AND fly - no collision at all."},
	{"Survival", "fly", "Fly without disabling collision."},
	{"Survival", "freeze", "Freeze everything except you."},

	{"Monsters", "kill monsters", "Kill every monster in the map."},
	{"Monsters", "mdk", "Kill whatever you are aiming at."},
	{"Monsters", "summon cyberdemon", "Spawn any actor in front of you."},
	{"Monsters", "dumpspawnables", "List every actor name you can summon - includes the mod's own."},

	{"Saves", "save mysave", "Save the game under a name."},
	{"Saves", "load mysave", "Load a named save."},
	{"Saves", "quicksave", "Quicksave."},
	{"Saves", "quickload", "Quickload."},

	{"View", "am_cheat 1", "Automap cheat: reveal the whole map. Try 2 to also show things."},
	{"View", "chase", "Third-person camera. Run again to go back."},
	{"View", "screenshot", "Take a screenshot."},
	{"View", "screenblocks 11", "Screen size / HUD layout, 3 to 12."},
	{"View", "hud_scale 2", "Scale the HUD up if it is too small."},

	{"Engine", "stat fps", "Show a framerate counter."},
	{"Engine", "vid_fps 1", "Alternative simple FPS display."},
	{"Engine", "gameversion", "Print the engine version."},
	{"Engine", "snd_reset", "Restart the sound system if audio breaks."},
	{"Engine", "logfile doom.log", "Write everything the console prints to a file."},
	{"Engine", "puke 3", "Run ACS script number 3 - for debugging map scripts."},
	{"Engine", `alias maxammo "give backpack; give ammo"`, "Make your own command out of several. Then bind it to a key: bind m maxammo - now M tops you up instantly."},
	{"Engine", "bind m maxammo", "Bind any command or alias to a key. Use defaultbind to see what a key does normally."},
	{"Engine", "sv_cheats 1", "Allow cheats in a netgame (not needed in single player)."},
}

// cheatModel backs the Cheatsheet table and holds the current filter.
type cheatModel struct {
	walk.TableModelBase
	shown []cheatItem
}

func newCheatModel() *cheatModel {
	return &cheatModel{shown: cheatItems}
}

func (m *cheatModel) RowCount() int { return len(m.shown) }

func (m *cheatModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.shown) {
		return ""
	}
	switch col {
	case 0:
		return m.shown[row].Category
	case 1:
		return m.shown[row].Command
	case 2:
		return m.shown[row].Desc
	}
	return ""
}

func (m *cheatModel) at(row int) (cheatItem, bool) {
	if row < 0 || row >= len(m.shown) {
		return cheatItem{}, false
	}
	return m.shown[row], true
}

// filter matches against command, description and category at once, so typing
// "key" or "inventory" finds things by what they do, not just by name.
func (m *cheatModel) filter(q string) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		m.shown = cheatItems
		m.PublishRowsReset()
		return
	}
	var out []cheatItem
	for _, c := range cheatItems {
		hay := strings.ToLower(c.Command + " " + c.Desc + " " + c.Category)
		if strings.Contains(hay, q) {
			out = append(out, c)
		}
	}
	m.shown = out
	m.PublishRowsReset()
}
