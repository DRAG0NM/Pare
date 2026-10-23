package rules

import "testing"

func TestRuleMatchesOnlyInsideItsDirectory(t *testing.T) {
	rule := Builtin()[0]
	tests := map[string]bool{
		`C:\Users\me\AppData\Local\Temp\a.tmp`:        true,
		`c:\users\Me\appdata\local\temp\sub\b.log`:    true,
		`\\?\C:\Users\me\AppData\Local\Temp\c.tmp`:    true,
		`C:\Users\me\AppData\Local\Temp`:              false,
		`C:\Users\me\AppData\Local\TempFiles\a.tmp`:   false,
		`C:\Users\me\x\AppData\Local\Temp\a.tmp`:      false,
		`C:\Windows\Temp\a.tmp`:                       false,
		`D:\Backup\Users\me\AppData\Local\Temp\a.tmp`: false,
	}
	for path, want := range tests {
		if got := rule.Matches(path); got != want {
			t.Errorf("Matches(%q) = %v, want %v", path, got, want)
		}
	}
}
