package plan

import (
	"path/filepath"
	"strings"
)

// protectedTop are entries directly under a volume root that are never quarantined.
var protectedTop = map[string]string{
	"windows":                   "Windows 系统目录",
	"system volume information": "系统还原和卷信息",
	"$recycle.bin":              "回收站（请用系统清空）",
	"recovery":                  "恢复环境",
	"boot":                      "启动文件",
	"efi":                       "启动文件",
	"$winreagent":               "恢复环境",
	"$windows.~bt":              "系统升级文件",
	"$windows.~ws":              "系统升级文件",
	"config.msi":                "安装程序回滚数据",
	"pagefile.sys":              "页面文件（请在系统设置中调整）",
	"hiberfil.sys":              "休眠文件（请用 powercfg 关闭休眠）",
	"swapfile.sys":              "交换文件",
	"bootmgr":                   "启动文件",
	".pare-quarantine":          "Pare 隔离区",
}

// windowsAllowed are directories under Windows whose contents (not the directories
// themselves) may be quarantined: temporary files, update downloads and crash dumps.
var windowsAllowed = []string{
	`windows\temp\`,
	`windows\systemtemp\`,
	`windows\softwaredistribution\download\`,
	`windows\livekernelreports\`,
	`windows\minidump\`,
	`windows\lastgood.tmp\`,
}

// protectedExact are folders that must stay even though their contents may go.
var protectedExact = map[string]bool{
	"users": true, "program files": true, "program files (x86)": true, "programdata": true,
}

var profileFolders = map[string]bool{
	"appdata": true, "desktop": true, "documents": true, "downloads": true, "pictures": true,
	"videos": true, "music": true, "favorites": true, "onedrive": true, "saved games": true,
}

// Protected reports whether path must never be quarantined, whatever a rule, the AI or
// the user selected, and why. It is checked when a plan is built and again right before
// a move.
func Protected(path string) (bool, string) {
	clean := filepath.Clean(strings.TrimPrefix(path, `\\?\`))
	volume := filepath.VolumeName(clean)
	if volume == "" || !filepath.IsAbs(clean) {
		return true, "不是完整的绝对路径"
	}
	rest := strings.ToLower(strings.Trim(strings.TrimPrefix(clean, volume), `\`))
	if rest == "" {
		return true, "卷根目录"
	}
	parts := strings.Split(rest, `\`)
	if reason, ok := protectedTop[parts[0]]; ok {
		if parts[0] == "windows" {
			for _, allowed := range windowsAllowed {
				if strings.HasPrefix(rest+`\`, allowed) && len(rest)+1 > len(allowed) {
					return false, ""
				}
			}
		}
		return true, reason
	}
	if len(parts) == 1 && protectedExact[parts[0]] {
		return true, "系统必需的顶层目录"
	}
	switch parts[0] {
	case "users":
		if len(parts) == 2 {
			return true, "用户配置文件根目录"
		}
		if len(parts) == 3 && profileFolders[parts[2]] {
			return true, "用户文件夹本身（可以处理其中的内容）"
		}
		if len(parts) == 4 && parts[2] == "appdata" {
			return true, "AppData 根目录"
		}
		if len(parts) == 3 && strings.HasPrefix(parts[2], "ntuser") {
			return true, "用户注册表文件"
		}
	case "program files", "program files (x86)":
		if len(parts) >= 2 && parts[1] == "windowsapps" {
			return true, "应用商店程序目录"
		}
	case "programdata":
		if len(parts) >= 2 && parts[1] == "microsoft" {
			return true, "Windows 系统数据"
		}
	}
	return false, ""
}
