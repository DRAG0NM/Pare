package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// diagnostic is one allowlisted read-only command. Arguments are passed to the process
// as an argv array, never through a shell, and every user-supplied value is validated.
type diagnostic struct {
	description string
	timeout     time.Duration
	build       func(distro, path string) ([]string, error)
}

var distroName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func needDistro(distro string) error {
	if !distroName.MatchString(distro) {
		return fmt.Errorf("发行版名 %q 不合法", distro)
	}
	return nil
}

// linuxPath accepts an absolute Linux path without control characters or option-like
// prefixes.
func linuxPath(path string) error {
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\x00\r\n") || strings.Contains(path, "/-") {
		return fmt.Errorf("Linux 路径 %q 不合法，需要以 / 开头的绝对路径", path)
	}
	return nil
}

func windowsAbsolute(path string) error {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n\"") {
		return fmt.Errorf("路径 %q 不合法，需要 Windows 绝对路径", path)
	}
	return nil
}

var diagnostics = map[string]diagnostic{
	"wsl_list": {"列出 WSL 发行版、状态和版本", time.Minute, func(string, string) ([]string, error) {
		return []string{"wsl.exe", "--list", "--verbose"}, nil
	}},
	"wsl_df": {"WSL 发行版内各文件系统的容量和已用（会启动该发行版）", 2 * time.Minute, func(distro, _ string) ([]string, error) {
		return []string{"wsl.exe", "-d", distro, "-e", "df", "-h"}, needDistro(distro)
	}},
	"wsl_du": {"WSL 内某个目录下各子目录的大小", 5 * time.Minute, func(distro, path string) ([]string, error) {
		if err := needDistro(distro); err != nil {
			return nil, err
		}
		return []string{"wsl.exe", "-d", distro, "-e", "du", "-xh", "--max-depth=1", "--", path}, linuxPath(path)
	}},
	"wsl_ls": {"列出 WSL 内某个目录", time.Minute, func(distro, path string) ([]string, error) {
		if err := needDistro(distro); err != nil {
			return nil, err
		}
		return []string{"wsl.exe", "-d", distro, "-e", "ls", "-la", "--", path}, linuxPath(path)
	}},
	"wsl_head": {"读取 WSL 内文本文件开头 8 KB", time.Minute, func(distro, path string) ([]string, error) {
		if err := needDistro(distro); err != nil {
			return nil, err
		}
		return []string{"wsl.exe", "-d", distro, "-e", "head", "-c", "8192", "--", path}, linuxPath(path)
	}},
	"wsl_journal": {"WSL 内 systemd 日志占用", time.Minute, func(distro, _ string) ([]string, error) {
		return []string{"wsl.exe", "-d", distro, "-e", "journalctl", "--disk-usage"}, needDistro(distro)
	}},
	"sparse_flag": {"查看文件是否为稀疏文件（如 WSL 的 ext4.vhdx）", time.Minute, func(_, path string) ([]string, error) {
		return []string{"fsutil.exe", "sparse", "queryflag", path}, windowsAbsolute(path)
	}},
	"shadow_storage": {"卷影副本（系统还原点）占用", time.Minute, func(string, string) ([]string, error) {
		return []string{"vssadmin.exe", "list", "shadowstorage"}, nil
	}},
	"driver_packages": {"驱动程序包列表（用于找出重复的旧驱动）", 2 * time.Minute, func(string, string) ([]string, error) {
		return []string{"pnputil.exe", "/enum-drivers"}, nil
	}},
	"component_store": {"分析 WinSxS 组件存储可回收的空间（需要管理员，可能要几分钟）", 10 * time.Minute, func(string, string) ([]string, error) {
		return []string{"Dism.exe", "/Online", "/Cleanup-Image", "/AnalyzeComponentStore"}, nil
	}},
	"docker_df": {"Docker 镜像、容器、卷和构建缓存的占用", 2 * time.Minute, func(string, string) ([]string, error) {
		return []string{"docker", "system", "df"}, nil
	}},
	"power_states": {"睡眠和休眠状态（休眠文件 hiberfil.sys 是否启用）", time.Minute, func(string, string) ([]string, error) {
		return []string{"powercfg.exe", "/a"}, nil
	}},
}

func diagnosticNames() string {
	var names []string
	for name, d := range diagnostics {
		names = append(names, name+"（"+d.description+"）")
	}
	sort.Strings(names)
	return strings.Join(names, "；")
}

// wslSensitivePath maps a Linux path onto IsSensitive's Windows-style patterns.
func wslSensitivePath(path string) bool {
	return IsSensitive(strings.ReplaceAll(path, "/", `\`))
}

func (e *Environment) diagnostic(ctx context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Name   string `json:"name"`
		Distro string `json:"distro"`
		Path   string `json:"path"`
	}](raw)
	if err != nil {
		return "", err
	}
	d, ok := diagnostics[args.Name]
	if !ok {
		return "", fmt.Errorf("没有名为 %q 的诊断命令；可用：%s", args.Name, diagnosticNames())
	}
	argv, err := d.build(args.Distro, args.Path)
	if err != nil {
		return "", err
	}
	if args.Name == "wsl_head" && wslSensitivePath(args.Path) {
		if e.Consent == nil || !e.Consent(ctx, args.Distro+":"+args.Path, "读取 WSL 内文件内容") {
			return "", errDenied
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	output, err := runCommand(runCtx, argv)
	text := strings.TrimSpace(decodeText(output))
	if err != nil {
		return fmt.Sprintf("命令 %s 失败：%v\n%s", strings.Join(argv, " "), err, text), nil
	}
	return "$ " + strings.Join(argv, " ") + "\n" + text, nil
}
