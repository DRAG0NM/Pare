package plan

import "testing"

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
