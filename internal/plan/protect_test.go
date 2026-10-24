package plan

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
)

func TestSelectingAContentsOnlyFolderExpandsIt(t *testing.T) {
	temp := `C:\Windows\SystemTemp`
	scan := model.ScanReport{Nodes: []model.Node{
		{Path: temp, ParentPath: `C:\Windows`, Kind: "directory", FileID: "d"},
		file(temp+`\fff2-e083`, "1", 30*24*time.Hour),
		file(temp+`\e2e0-c98b`, "2", 30*24*time.Hour),
		file(`C:\Windows\System32\kernel32.dll`, "3", 30*24*time.Hour),
	}}
	for i := range scan.Nodes[1:] {
		scan.Nodes[i+1].ParentPath = filepath.Dir(scan.Nodes[i+1].Path)
	}

	result, err := BuildSelected(context.Background(), scan, []Selection{{Path: temp, Source: "AI 建议：更新包残留"}}, &fakeProber{status: model.EvidencePass}, now, time.Hour)

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || !result.Items[0].Executable || !result.Items[1].Executable || !strings.Contains(result.Items[0].Evidence[0].Detail, "SystemTemp 中的内容") {
		t.Fatalf("items = %+v; want the two files inside SystemTemp, not the folder", result.Items)
	}
	if !ContentsOnly(`c:\windows\systemtemp\`) || ContentsOnly(`C:\Windows\SystemTemp\x`) || ContentsOnly(`C:\Windows`) {
		t.Error("ContentsOnly matches the wrong paths")
	}
}

func TestProtected(t *testing.T) {
	for path, want := range map[string]bool{
		`C:\`:                                               true,
		`C:\Windows`:                                        true,
		`C:\Windows\System32\drivers\etc\hosts`:             true,
		`C:\Windows\SystemTemp`:                             true,
		`C:\Windows\SystemTemp\fff2-e083`:                   false,
		`C:\Windows\Temp\x.log`:                             false,
		`C:\WINDOWS\SoftwareDistribution\Download\a`:        false,
		`C:\Windows\SoftwareDistribution\DataStore`:         true,
		`C:\System Volume Information`:                      true,
		`D:\$RECYCLE.BIN\S-1-5-21\x`:                        true,
		`C:\pagefile.sys`:                                   true,
		`C:\Users`:                                          true,
		`C:\Users\me`:                                       true,
		`C:\Users\me\Downloads`:                             true,
		`C:\Users\me\Downloads\setup.exe`:                   false,
		`C:\Users\me\AppData\Local`:                         true,
		`C:\Users\me\AppData\Local\npm-cache\_npx`:          false,
		`C:\Users\me\NTUSER.DAT`:                            true,
		`C:\Program Files`:                                  true,
		`C:\Program Files\WindowsApps\x`:                    true,
		`C:\Program Files\QuarkUpdater\old`:                 false,
		`C:\ProgramData\Microsoft\Windows\WER`:              true,
		`C:\ProgramData\Epic\EpicGamesLauncher\Data\Update`: false,
		`C:\.pare-quarantine\batch`:                         true,
		`relative\path`:                                     true,
		`\\?\C:\Windows\Temp\a`:                             false,
	} {
		if got, reason := Protected(path); got != want {
			t.Errorf("Protected(%q) = %v (%s), want %v", path, got, reason, want)
		}
	}
}
