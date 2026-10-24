package advisor

import (
	"path/filepath"
	"strings"
)

// sensitiveDirs are path fragments (lower case, backslash separated) whose files hold
// credentials or private keys. Reading their content needs the user's consent; names and
// sizes stay available.
var sensitiveDirs = []string{
	`\.ssh\`,
	`\.gnupg\`,
	`\.aws\`,
	`\.azure\`,
	`\.kube\`,
	`\.docker\config.json`,
	`\microsoft\credentials\`,
	`\microsoft\protect\`,
	`\microsoft\vault\`,
	`\microsoft\crypto\`,
	`\1password\`,
	`\bitwarden\`,
	`\keepass`,
	`\pare\advisor.json`,
}

// sensitiveNames match the file name (lower case). A trailing * matches any suffix.
var sensitiveNames = []string{
	"id_rsa*", "id_dsa*", "id_ecdsa*", "id_ed25519*",
	".env", ".env.*", ".netrc", "_netrc", ".git-credentials", ".npmrc", ".pypirc",
	"login data", "login data-journal", "cookies", "cookies-journal", "web data", "local state",
	"logins.json", "key3.db", "key4.db", "cookies.sqlite", "cert9.db",
	"credentials", "credentials.json", "secrets.json", "token.json",
}

var sensitiveExtensions = map[string]bool{
	".pem": true, ".key": true, ".pfx": true, ".p12": true, ".kdbx": true, ".ppk": true, ".jks": true, ".keystore": true,
}

// IsSensitive reports whether reading path's content should first ask the user.
func IsSensitive(path string) bool {
	lower := strings.ToLower(filepath.Clean(path))
	for _, fragment := range sensitiveDirs {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	name := filepath.Base(lower)
	if sensitiveExtensions[filepath.Ext(name)] {
		return true
	}
	for _, pattern := range sensitiveNames {
		if prefix, wildcard := strings.CutSuffix(pattern, "*"); wildcard {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		} else if name == pattern {
			return true
		}
	}
	return false
}
