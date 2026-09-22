package main

import (
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type presetResult struct {
	OK      bool
	Deleted bool
	Name    string
	Files   []string
}

// relToGameDir keeps files that live in the game folder as bare names, so the
// whole setup stays portable if the folder ever moves.
func relToGameDir(gameDir, p string) string {
	if rel, err := filepath.Rel(gameDir, p); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}

// runPresetDialog edits one campaign entry. Load order is the whole point here,
// so the file list is explicitly reorderable rather than however the file
// picker happened to hand the selection back.
func runPresetDialog(owner walk.Form, gameDir, name string, files []string, allowDelete bool) presetResult {
	res := presetResult{Name: name, Files: append([]string{}, files...)}

	var dlg *walk.Dialog
	var nameEdit *walk.LineEdit
	var list *walk.ListBox
	var okPB, cancelPB *walk.PushButton

	refresh := func(sel int) {
		list.SetModel(append([]string{}, res.Files...))
		if sel >= 0 && sel < len(res.Files) {
			list.SetCurrentIndex(sel)
		}
	}
	move := func(delta int) {
		i := list.CurrentIndex()
		j := i + delta
		if i < 0 || j < 0 || j >= len(res.Files) {
			return
		}
		res.Files[i], res.Files[j] = res.Files[j], res.Files[i]
		refresh(j)
	}

	title := "Edit campaign"
	if name == "" {
		title = "New campaign"
	}

	cmd, err := Dialog{
		AssignTo:      &dlg,
		Title:         title,
		DefaultButton: &okPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{Width: 520, Height: 340},
		Layout:        VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{Text: "Name:"},
					LineEdit{AssignTo: &nameEdit, Text: name},
				},
			},
			Label{Text: "Files, in load order (first line loads first, Brutal Doom is added after these):"},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					ListBox{AssignTo: &list, Model: append([]string{}, res.Files...)},
					Composite{
						Layout:  VBox{MarginsZero: true},
						MaxSize: Size{Width: 110},
						Children: []Widget{
							PushButton{Text: "Add files...", OnClicked: func() {
								fd := &walk.FileDialog{
									Title:          "Add files to this campaign",
									InitialDirPath: gameDir,
									Filter:         "Doom files (*.wad;*.pk3;*.pk7;*.deh)|*.wad;*.pk3;*.pk7;*.deh|All files (*.*)|*.*",
								}
								if ok, err := fd.ShowOpenMultiple(dlg); err != nil || !ok {
									return
								}
								for _, p := range fd.FilePaths {
									res.Files = append(res.Files, relToGameDir(gameDir, p))
								}
								if nameEdit.Text() == "" && len(res.Files) > 0 {
									b := filepath.Base(res.Files[0])
									nameEdit.SetText(strings.TrimSpace(strings.TrimSuffix(b, filepath.Ext(b))))
								}
								refresh(len(res.Files) - 1)
							}},
							PushButton{Text: "Move up", OnClicked: func() { move(-1) }},
							PushButton{Text: "Move down", OnClicked: func() { move(1) }},
							PushButton{Text: "Remove", OnClicked: func() {
								i := list.CurrentIndex()
								if i < 0 || i >= len(res.Files) {
									return
								}
								res.Files = append(res.Files[:i], res.Files[i+1:]...)
								refresh(i - 1)
							}},
							VSpacer{},
						},
					},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{
						Text:    "Delete",
						Visible: allowDelete,
						MaxSize: Size{Width: 90},
						OnClicked: func() {
							if walk.MsgBox(dlg, "Delete campaign",
								"Remove \""+name+"\" from the list?\n\nThe WAD files themselves are not touched.",
								walk.MsgBoxIconQuestion|walk.MsgBoxYesNo) == walk.DlgCmdYes {
								res.Deleted = true
								dlg.Accept()
							}
						},
					},
					HSpacer{},
					PushButton{AssignTo: &okPB, Text: "OK", MaxSize: Size{Width: 90}, OnClicked: func() {
						if strings.TrimSpace(nameEdit.Text()) == "" {
							walk.MsgBox(dlg, "Campaign", "Give this campaign a name.", walk.MsgBoxIconWarning)
							return
						}
						if len(res.Files) == 0 {
							walk.MsgBox(dlg, "Campaign", "Add at least one file.", walk.MsgBoxIconWarning)
							return
						}
						res.Name = strings.TrimSpace(nameEdit.Text())
						dlg.Accept()
					}},
					PushButton{AssignTo: &cancelPB, Text: "Cancel", MaxSize: Size{Width: 90}, OnClicked: func() {
						dlg.Cancel()
					}},
				},
			},
		},
	}.Run(owner)

	res.OK = err == nil && cmd == walk.DlgCmdOK
	return res
}
