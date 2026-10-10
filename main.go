package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// version is injected at build time by build.ps1, which derives it from the
// commit count and hash so two machines can be compared at a glance. A plain
// "go build" leaves it as "dev".
var version = "dev"

func windowTitle() string {
	if version == "dev" {
		return "Doom Launcher (dev build)"
	}
	return "Doom Launcher " + version
}

var vanillaSkills = []string{
	"Default",
	"1 - I'm Too Young To Die",
	"2 - Hey, Not Too Rough",
	"3 - Hurt Me Plenty",
	"4 - Ultra-Violence",
	"5 - Nightmare!",
}

// brutalSkills are the eight skill definitions in Brutal Doom's MAPINFO, which
// replace the stock five whenever the mod is loaded. Skills 6-8 are unreachable
// without it, and the names for 1-5 are the mod's own.
var brutalSkills = []string{
	"Default",
	"1 - I'm Too Young To Die",
	"2 - Hey, Not Too Rough",
	"3 - Hurt Me Plenty",
	"4 - Ultra-Violence",
	"5 - Nightmare!",
	"6 - Black Metal",
	"7 - Realism Violence",
	"8 - Realism Nightmare",
}

// skills returns the list that matches what will actually be loaded.
func (l *launcher) skills() []string {
	if l.brutalBox != nil && l.brutalBox.Checked() {
		return brutalSkills
	}
	return vanillaSkills
}

type launcher struct {
	gameDir string
	cfgPath string
	cfg     *Config
	scan    scanResult
	modFile string
	hist    *historyModel

	mw        *walk.MainWindow
	tabs      *walk.TabWidget
	iwadBox   *walk.ComboBox
	campBoxes [4]*walk.ComboBox
	buckets   [4][]Entry
	skillBox  *walk.ComboBox
	extraEdit *walk.LineEdit
	brutalBox *walk.CheckBox
	preview   *walk.TextEdit
	histTable *walk.TableView
	histCmd   *walk.TextEdit

	wads       *wadCache
	wadBox     *walk.TextEdit
	wadGen     int
	readmeBox  *walk.TextEdit
	readmeName *walk.Label
	readmeFile string

	statusBox   *walk.ComboBox
	statuses    *statusModel
	statusTable *walk.TableView

	sbCounts   *walk.StatusBarItem
	sbProgress *walk.StatusBarItem
	sbEngine   *walk.StatusBarItem
	engineVer  string
	sbSize     *walk.StatusBarItem

	cheats      *cheatModel
	cheatFilter *walk.LineEdit
	cheatTable  *walk.TableView
	cheatDetail *walk.TextEdit

	loading bool
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fatal("Could not determine the launcher's own location: " + err.Error())
	}
	gameDir := filepath.Dir(exe)

	l := &launcher{gameDir: gameDir}
	l.cfgPath = configPath(gameDir)
	l.cfg = loadConfig(l.cfgPath)
	l.hist = loadHistory(historyPath(l.cfgPath))
	l.cheats = newCheatModel()
	l.wads = newWadCache()
	l.engineVer = engineVersion(l.abs(l.cfg.Engine))
	l.statuses = &statusModel{l: l}
	l.rescan()

	if err := l.build(); err != nil {
		fatal("Could not create the window: " + err.Error())
	}
	l.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		l.persist()
	})
	l.mw.Run()
}

func fatal(msg string) {
	walk.MsgBox(nil, "Doom Launcher", msg, walk.MsgBoxIconError)
	os.Exit(1)
}

func (l *launcher) rescan() {
	l.scan = scanDir(l.gameDir, l.cfg.Presets)
	l.nameEntries()
	l.bucketize()

	// Honour an explicitly configured mod file while it is still present,
	// otherwise fall back to whatever Brutal Doom build is in the folder.
	l.modFile = ""
	for _, m := range l.scan.Mods {
		if strings.EqualFold(m, l.cfg.ModFile) {
			l.modFile = m
		}
	}
	if l.modFile == "" && len(l.scan.Mods) > 0 {
		l.modFile = l.scan.Mods[len(l.scan.Mods)-1]
	}
}

func (l *launcher) build() error {
	brutalLabel := "Brutal Doom  (no brutal*.pk3 found in this folder)"
	if l.modFile != "" {
		brutalLabel = "Brutal Doom  (" + l.modFile + ")"
	}

	err := MainWindow{
		AssignTo: &l.mw,
		Title:    windowTitle(),
		MinSize:  Size{Width: 700, Height: 470},
		Size:     Size{Width: 780, Height: 540},
		Layout:   VBox{},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &l.sbEngine, Width: 140},
			{AssignTo: &l.sbCounts, Width: 260},
			{AssignTo: &l.sbProgress, Width: 330},
			{AssignTo: &l.sbSize, Width: 80},
		},
		Children: []Widget{
			TabWidget{
				AssignTo: &l.tabs,
				Pages: []TabPage{
					{
						Title:  "Launch",
						Layout: VBox{},
						Children: []Widget{
							Composite{
								Layout: Grid{Columns: 3},
								Children: []Widget{
									Label{Text: "Base game (IWAD):"},
									ComboBox{
										AssignTo:              &l.iwadBox,
										Model:                 l.scan.IWADs,
										OnCurrentIndexChanged: l.onIWADChanged,
									},
									HSpacer{Size: 10},

									GroupBox{
										Title:      "Campaign / WAD  -  choose one; the other three clear themselves",
										ColumnSpan: 3,
										Layout:     Grid{Columns: 2},
										Children: []Widget{
											Label{Text: "In progress:"},
											ComboBox{
												AssignTo:              &l.campBoxes[0],
												Model:                 l.bucketNames(0),
												OnCurrentIndexChanged: l.onCampaignChanged(0),
											},
											Label{Text: "Not started:"},
											ComboBox{
												AssignTo:              &l.campBoxes[1],
												Model:                 l.bucketNames(1),
												OnCurrentIndexChanged: l.onCampaignChanged(1),
											},
											Label{Text: "Completed:"},
											ComboBox{
												AssignTo:              &l.campBoxes[2],
												Model:                 l.bucketNames(2),
												OnCurrentIndexChanged: l.onCampaignChanged(2),
											},
											Label{Text: "Ignored:"},
											ComboBox{
												AssignTo:              &l.campBoxes[3],
												Model:                 l.bucketNames(3),
												OnCurrentIndexChanged: l.onCampaignChanged(3),
											},
											Label{Text: ""},
											Composite{
												Layout: HBox{MarginsZero: true},
												Children: []Widget{
													PushButton{Text: "New campaign...", MinSize: Size{Width: 120}, OnClicked: l.onAddPreset},
													PushButton{Text: "Edit selected...", MinSize: Size{Width: 120}, OnClicked: l.onEditPreset},
													HSpacer{Size: 16},
													Label{Text: "Status:"},
													ComboBox{
														AssignTo:              &l.statusBox,
														Model:                 statusLabels(),
														MinSize:               Size{Width: 120},
														OnCurrentIndexChanged: l.onStatusBoxChanged,
													},
													HSpacer{},
												},
											},
										},
									},

									Label{Text: "Skill:"},
									ComboBox{
										AssignTo:              &l.skillBox,
										Model:                 vanillaSkills,
										OnCurrentIndexChanged: l.updatePreview,
									},
									HSpacer{Size: 10},

									Label{Text: "Extra arguments:"},
									LineEdit{
										AssignTo:      &l.extraEdit,
										CueBanner:     "e.g.  -warp 07   +freelook 1",
										OnTextChanged: l.updatePreview,
									},
									HSpacer{Size: 10},
								},
							},
							CheckBox{
								AssignTo:         &l.brutalBox,
								Text:             brutalLabel,
								Enabled:          l.modFile != "",
								OnCheckedChanged: l.onBrutalChanged,
							},
							Label{Text: "Selected WAD:"},
							TextEdit{
								AssignTo: &l.wadBox,
								ReadOnly: true,
								VScroll:  true,
								MinSize:  Size{Height: 64},
							},
							Label{Text: "Command line:"},
							TextEdit{
								AssignTo: &l.preview,
								ReadOnly: true,
								VScroll:  true,
								MinSize:  Size{Height: 48},
							},
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									PushButton{Text: "Refresh folder", MaxSize: Size{Width: 120}, OnClicked: l.onRefresh},
									HSpacer{},
									PushButton{Text: "Launch", MinSize: Size{Width: 120, Height: 32}, OnClicked: l.onLaunch},
								},
							},
						},
					},
					{
						Title:  "Progress",
						Layout: VBox{},
						Children: []Widget{
							TableView{
								AssignTo:            &l.statusTable,
								Model:               l.statuses,
								AlternatingRowBG:    true,
								MultiSelection:      true,
								ColumnsOrderable:    false,
								LastColumnStretched: true,
								Columns: []TableViewColumn{
									{Title: "Title", Width: 230},
									{Title: "File", Width: 150},
									{Title: "Status", Width: 100},
									{Title: "Loads", Width: 260},
								},
							},
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "Set selected to:"},
									PushButton{Text: "In progress", OnClicked: func() { l.setSelectedStatus(StatusProgress) }},
									PushButton{Text: "Not started", OnClicked: func() { l.setSelectedStatus(StatusTodo) }},
									PushButton{Text: "Completed", OnClicked: func() { l.setSelectedStatus(StatusDone) }},
									PushButton{Text: "Ignored", OnClicked: func() { l.setSelectedStatus(StatusIgnore) }},
									HSpacer{},
								},
							},
							Label{Text: "Ctrl-click or shift-click to select several. Status decides which dropdown a campaign appears in on the Launch tab."},
						},
					},
					{
						Title:  "History",
						Layout: VBox{},
						Children: []Widget{
							TableView{
								AssignTo:              &l.histTable,
								Model:                 l.hist,
								AlternatingRowBG:      true,
								ColumnsOrderable:      false,
								LastColumnStretched:   true,
								OnCurrentIndexChanged: l.onHistorySelect,
								OnItemActivated:       l.onHistoryUse,
								Columns: []TableViewColumn{
									{Title: "When", Width: 155},
									{Title: "Campaign", Width: 170},
									{Title: "Base game", Width: 100},
									{Title: "Brutal", Width: 55},
									{Title: "Skill", Width: 140},
									{Title: "Extra args", Width: 130},
								},
							},
							Label{Text: "Command line used (double-click a row to load it back):"},
							TextEdit{
								AssignTo: &l.histCmd,
								ReadOnly: true,
								VScroll:  true,
								MinSize:  Size{Height: 60},
							},
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									PushButton{Text: "Clear history", MaxSize: Size{Width: 120}, OnClicked: l.onHistoryClear},
									HSpacer{},
									PushButton{Text: "Use these settings", MaxSize: Size{Width: 150}, OnClicked: l.onHistoryUse},
								},
							},
						},
					},
					{
						Title:  "Readme",
						Layout: VBox{},
						Children: []Widget{
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									Label{AssignTo: &l.readmeName, Text: ""},
									HSpacer{},
									PushButton{Text: "Open in editor", MaxSize: Size{Width: 130}, OnClicked: l.onReadmeOpen},
								},
							},
							TextEdit{
								AssignTo: &l.readmeBox,
								ReadOnly: true,
								VScroll:  true,
								HScroll:  true,
								Font:     Font{Family: "Consolas", PointSize: 9},
							},
						},
					},
					{
						Title:  "Cheatsheet",
						Layout: VBox{},
						Children: []Widget{
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "Filter:"},
									LineEdit{
										AssignTo:      &l.cheatFilter,
										CueBanner:     "search by command or by what it does, e.g. keys",
										OnTextChanged: l.onCheatFilter,
									},
								},
							},
							TableView{
								AssignTo:              &l.cheatTable,
								Model:                 l.cheats,
								AlternatingRowBG:      true,
								ColumnsOrderable:      false,
								LastColumnStretched:   true,
								OnCurrentIndexChanged: l.onCheatSelect,
								OnItemActivated:       l.onCheatCopy,
								Columns: []TableViewColumn{
									{Title: "Group", Width: 80},
									{Title: "Command", Width: 175},
									{Title: "What it does", Width: 460},
								},
							},
							TextEdit{
								AssignTo: &l.cheatDetail,
								ReadOnly: true,
								VScroll:  true,
								MinSize:  Size{Height: 62},
							},
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "Open the console in game with the ` key (above Tab), then type the command."},
									HSpacer{},
									PushButton{Text: "Copy command", MaxSize: Size{Width: 140}, OnClicked: l.onCheatCopy},
								},
							},
						},
					},
				},
			},
		},
	}.Create()
	if err != nil {
		return err
	}

	if icon, err := walk.NewIconFromResourceId(appIconResourceID); err == nil {
		l.mw.SetIcon(icon)
	}
	l.restoreSelection()
	l.statuses.reload()
	l.updateStatusBar()
	l.syncStatusBox()
	return nil
}

// currentEntry finds the one dropdown that has a campaign selected. All four
// blank means no campaign, i.e. play the IWAD on its own.
func (l *launcher) currentEntry() Entry {
	for i := range l.campBoxes {
		if l.campBoxes[i] == nil {
			continue
		}
		if idx := l.campBoxes[i].CurrentIndex(); idx > 0 && idx-1 < len(l.buckets[i]) {
			return l.buckets[i][idx-1]
		}
	}
	return Entry{}
}

// restoreSelection puts every control back where it was left last session.
func (l *launcher) restoreSelection() {
	l.loading = true

	l.iwadBox.SetCurrentIndex(indexOfFold(l.scan.IWADs, l.cfg.LastIWAD, 0))
	l.selectCampaign(l.cfg.LastCampaign)

	l.syncBrutal()

	names := l.skills()
	l.skillBox.SetModel(names)
	skill := l.cfg.LastSkill
	if skill < 0 || skill >= len(names) {
		skill = 0
	}
	l.skillBox.SetCurrentIndex(skill)
	l.extraEdit.SetText(l.cfg.ExtraArgs)

	l.loading = false
	l.updatePreview()
	l.syncStatusBox()
}

func indexOfFold(list []string, want string, fallback int) int {
	for i, s := range list {
		if strings.EqualFold(s, want) {
			return i
		}
	}
	if fallback < len(list) {
		return fallback
	}
	return -1
}

func (l *launcher) currentIWAD() string {
	if l.iwadBox == nil {
		return ""
	}
	i := l.iwadBox.CurrentIndex()
	if i < 0 || i >= len(l.scan.IWADs) {
		return ""
	}
	return l.scan.IWADs[i]
}

// onCampaignChanged restores the IWAD this campaign was last played with, so
// switching to a Doom 1 megawad does not silently keep DOOM2.WAD selected.
// selectCampaign picks a campaign in whichever bucket holds it, blanking the
// other three.
func (l *launcher) selectCampaign(name string) {
	for i := range l.campBoxes {
		l.campBoxes[i].SetCurrentIndex(0)
	}
	if name == "" {
		return
	}
	for i := range l.buckets {
		for j, e := range l.buckets[i] {
			if strings.EqualFold(e.Name, name) {
				l.campBoxes[i].SetCurrentIndex(j + 1)
				return
			}
		}
	}
}

// onCampaignChanged keeps the four dropdowns mutually exclusive: choosing a
// campaign in one clears the others, so only ever one thing is loaded.
func (l *launcher) onCampaignChanged(which int) func() {
	return func() { l.campaignChanged(which) }
}

func (l *launcher) campaignChanged(which int) {
	if !l.loading {
		if l.campBoxes[which] != nil && l.campBoxes[which].CurrentIndex() > 0 {
			l.loading = true
			for j := range l.campBoxes {
				if j != which && l.campBoxes[j] != nil {
					l.campBoxes[j].SetCurrentIndex(0)
				}
			}
			l.loading = false
		}
		l.syncStatusBox()
		entry := l.currentEntry()
		if entry.IWAD != "" {
			// A base-game entry *is* its IWAD, so pin the base-game dropdown.
			if i := indexOfFold(l.scan.IWADs, entry.IWAD, -1); i >= 0 {
				l.loading = true
				l.iwadBox.SetCurrentIndex(i)
				l.loading = false
			}
		} else if iwad, ok := l.cfg.CampaignIWAD[strings.ToLower(entry.Name)]; ok {
			if i := indexOfFold(l.scan.IWADs, iwad, -1); i >= 0 {
				l.loading = true
				l.iwadBox.SetCurrentIndex(i)
				l.loading = false
			}
		}
	}
	l.updatePreview()
}

// onBrutalChanged swaps the skill list, keeping the selection where it still
// exists - unchecking the mod while on skill 7 falls back to the highest
// vanilla skill rather than silently launching something else.
func (l *launcher) onBrutalChanged() {
	if l.skillBox == nil {
		return
	}
	if l.brutalBox != nil && l.brutalBox.Enabled() {
		l.cfg.BrutalOn = boolPtr(l.brutalBox.Checked())
	}
	names := l.skills()
	cur := l.skillBox.CurrentIndex()
	if cur >= len(names) {
		cur = len(names) - 1
	}
	if cur < 0 {
		cur = 0
	}
	l.skillBox.SetModel(names)
	l.skillBox.SetCurrentIndex(cur)
	l.updatePreview()
}

func (l *launcher) onIWADChanged() {
	if !l.loading {
		iwad := l.currentIWAD()
		if entry := l.currentEntry(); entry.Name != "" {
			if l.clashesWith(entry, iwad) {
				// The pairing cannot work, so drop it rather than remember it.
				l.loading = true
				for i := range l.campBoxes {
					l.campBoxes[i].SetCurrentIndex(0)
				}
				l.loading = false
				l.syncStatusBox()
			} else {
				l.cfg.CampaignIWAD[strings.ToLower(entry.Name)] = iwad
			}
		}
		l.syncBrutal()
	}
	l.updatePreview()
}

// clashesWith reports whether a campaign simply cannot run on a base game, so
// the selection should be dropped instead of silently remembered as a pairing.
// Plausible combinations are left alone: Sunlust on TNT instead of Doom 2 is
// unusual but valid, and not the launcher's business to prevent.
func (l *launcher) clashesWith(entry Entry, iwad string) bool {
	if entry.IWAD != "" {
		// A base-game entry is its own IWAD; any other choice disagrees with it.
		return !strings.EqualFold(entry.IWAD, iwad)
	}
	// A campaign's maps tell us which generation of Doom it was built for.
	// Anything outside the Doom family cannot run a Doom campaign at all.
	if iwadFamily(iwad) != "doom" {
		return true
	}
	// Derive the primary file from the entry itself rather than from the
	// widgets, so this stays a pure decision about the pairing.
	var primary string
	for _, f := range entry.Files {
		if auxKind(f) == "" {
			primary = l.abs(f)
			break
		}
	}
	switch l.eraOf(primary) {
	case "Doom 1":
		return iwadEra(iwad) == "doom2"
	case "Doom 2":
		return iwadEra(iwad) == "doom1"
	}
	return false
}

func (l *launcher) spec() launchSpec {
	return launchSpec{
		IWAD:   l.currentIWAD(),
		Files:  l.currentEntry().Files,
		Mod:    l.modFile,
		Brutal: l.brutalBox.Checked(),
		Skill:  l.skillBox.CurrentIndex(),
		Extra:  l.extraEdit.Text(),
	}
}

func (l *launcher) updatePreview() {
	l.preview.SetText(commandLine(l.cfg.Engine, buildArgs(l.spec())))
	if l.wadBox != nil {
		l.updateWadInfo()
	}
}

func (l *launcher) persist() {
	l.cfg.LastIWAD = l.currentIWAD()
	l.cfg.LastCampaign = l.currentEntry().Name
	l.cfg.LastSkill = l.skillBox.CurrentIndex()
	l.cfg.ExtraArgs = l.extraEdit.Text()
	if l.brutalBox.Enabled() {
		l.cfg.BrutalOn = boolPtr(l.brutalBox.Checked())
	}
	if l.modFile != "" {
		l.cfg.ModFile = l.modFile
	}
	if name := l.currentEntry().Name; name != "" {
		l.cfg.CampaignIWAD[strings.ToLower(name)] = l.currentIWAD()
	}
	l.cfg.save(l.cfgPath)
}

func (l *launcher) onRefresh() {
	l.persist()
	l.rescan()
	l.loading = true
	l.iwadBox.SetModel(l.scan.IWADs)
	for i := range l.campBoxes {
		l.campBoxes[i].SetModel(l.bucketNames(i))
	}
	l.loading = false
	l.restoreSelection()
	l.statuses.reload()
	l.updateStatusBar()
}

// onAddPreset builds a new multi-file entry - the Extermination Day case, where
// several files have to load together in a fixed order.
func (l *launcher) onAddPreset() {
	l.applyPreset("", runPresetDialog(l.mw, l.gameDir, "", nil, false))
}

// onEditPreset edits the selected entry. Editing an auto-detected single-file
// entry simply promotes it into a saved preset.
func (l *launcher) onEditPreset() {
	entry := l.currentEntry()
	if entry.IWAD != "" {
		walk.MsgBox(l.mw, "Edit campaign",
			entry.Name+" is a base game, not a preset, so it has no files to edit."+
				crlf+crlf+"To change its status use the Status box beside these buttons, "+
				"or the Progress tab.",
			walk.MsgBoxIconInformation)
		return
	}
	if len(entry.Files) == 0 {
		walk.MsgBox(l.mw, "Edit campaign",
			"Pick a campaign in one of the four dropdowns first.\n\n"+
				"\"New campaign...\" builds a new multi-file entry from scratch.",
			walk.MsgBoxIconInformation)
		return
	}
	l.applyPreset(entry.Name, runPresetDialog(l.mw, l.gameDir, entry.Name, entry.Files, l.isPreset(entry.Name)))
}

func (l *launcher) isPreset(name string) bool {
	for _, p := range l.cfg.Presets {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

func (l *launcher) applyPreset(origName string, res presetResult) {
	if !res.OK {
		return
	}
	l.persist() // capture the current UI state before we rebuild the dropdown

	idx := -1
	for i, p := range l.cfg.Presets {
		if strings.EqualFold(p.Name, origName) {
			idx = i
			break
		}
	}

	switch {
	case res.Deleted:
		if idx >= 0 {
			l.cfg.Presets = append(l.cfg.Presets[:idx], l.cfg.Presets[idx+1:]...)
		}
		l.cfg.LastCampaign = ""
	case idx >= 0:
		l.cfg.Presets[idx] = Preset{Name: res.Name, Files: res.Files}
		l.cfg.LastCampaign = res.Name
	default:
		l.cfg.Presets = append(l.cfg.Presets, Preset{Name: res.Name, Files: res.Files})
		l.cfg.LastCampaign = res.Name
	}
	l.cfg.save(l.cfgPath)

	l.rescan()
	l.loading = true
	for i := range l.campBoxes {
		l.campBoxes[i].SetModel(l.bucketNames(i))
	}
	l.loading = false
	l.restoreSelection()
	l.statuses.reload()
	l.updateStatusBar()
}

func (l *launcher) onCheatFilter() {
	l.cheats.filter(l.cheatFilter.Text())
	l.cheatDetail.SetText("")
}

func (l *launcher) onCheatSelect() {
	if c, ok := l.cheats.at(l.cheatTable.CurrentIndex()); ok {
		l.cheatDetail.SetText(c.Command + "\r\n\r\n" + c.Desc)
	} else {
		l.cheatDetail.SetText("")
	}
}

// onCheatCopy puts the command on the clipboard - the console has no paste, but
// this is still the quickest way to get it into a bind or a note.
func (l *launcher) onCheatCopy() {
	if c, ok := l.cheats.at(l.cheatTable.CurrentIndex()); ok {
		walk.Clipboard().SetText(c.Command)
	}
}

func (l *launcher) onHistorySelect() {
	if e, ok := l.hist.at(l.histTable.CurrentIndex()); ok {
		l.histCmd.SetText(e.Command)
	} else {
		l.histCmd.SetText("")
	}
}

// onHistoryUse replays an old launch by restoring its settings onto the
// Launch tab, rather than firing the game off immediately.
func (l *launcher) onHistoryUse() {
	e, ok := l.hist.at(l.histTable.CurrentIndex())
	if !ok {
		return
	}

	l.loading = true
	if i := indexOfFold(l.scan.IWADs, e.IWAD, -1); i >= 0 {
		l.iwadBox.SetCurrentIndex(i)
	}
	l.selectCampaign(e.Campaign)
	l.brutalBox.SetChecked(e.Brutal && l.modFile != "")
	names := l.skills()
	l.skillBox.SetModel(names)
	if e.Skill >= 0 && e.Skill < len(names) {
		l.skillBox.SetCurrentIndex(e.Skill)
	}
	l.extraEdit.SetText(e.Extra)
	l.loading = false

	l.updatePreview()
	l.tabs.SetCurrentIndex(0)
}

func (l *launcher) onHistoryClear() {
	if walk.MsgBox(l.mw, "Clear history", "Delete every recorded launch?",
		walk.MsgBoxIconQuestion|walk.MsgBoxYesNo) == walk.DlgCmdYes {
		l.hist.clear()
		l.histCmd.SetText("")
	}
}

func (l *launcher) onLaunch() {
	l.persist()

	engine := l.cfg.Engine
	if !filepath.IsAbs(engine) {
		engine = filepath.Join(l.gameDir, engine)
	}
	if _, err := os.Stat(engine); err != nil {
		walk.MsgBox(l.mw, "Doom Launcher",
			"Could not find the engine:\n\n"+engine+
				"\n\nSet \"engine\" in launcher.json if it lives somewhere else.",
			walk.MsgBoxIconError)
		return
	}

	if msg := l.unplayableReason(); msg != "" {
		walk.MsgBox(l.mw, "Cannot launch", msg, walk.MsgBoxIconError)
		return
	}

	spec := l.spec()
	args := buildArgs(spec)

	cmd := exec.Command(engine, args...)
	cmd.Dir = l.gameDir
	if err := cmd.Start(); err != nil {
		walk.MsgBox(l.mw, "Doom Launcher", "Failed to start:\n\n"+err.Error(), walk.MsgBoxIconError)
		return
	}

	// Playing something marks it as started, unless you have already filed it
	// as completed or ignored.
	if l.promoteOnLaunch(l.currentEntry().Name) {
		l.cfg.save(l.cfgPath)
	}

	l.hist.add(HistoryEntry{
		Time:     time.Now(),
		IWAD:     spec.IWAD,
		Campaign: l.currentEntry().Name,
		Brutal:   spec.Brutal && l.modFile != "",
		Skill:    spec.Skill,
		Extra:    spec.Extra,
		Command:  commandLine(l.cfg.Engine, args),
	})

	l.mw.Close()
}
