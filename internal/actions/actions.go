// Package actions prepares and runs the supported action templates the AI may propose,
// and commands the user explicitly chose to run. Templates validate every parameter and
// run as argv arrays; nothing here runs without the user's confirmation in the UI.
package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/winexec"
)

// Prepared is an action ready to show and run: the exact steps, and what the user must
// know before confirming.
type Prepared struct {
	Template string     `json:"template"`
	Steps    [][]string `json:"steps"`
	// Shell holds a raw command line for non-template commands; Steps is empty then.
	Shell      string   `json:"shell,omitempty"`
	Warnings   []string `json:"warnings"`
	NeedsAdmin bool     `json:"needsAdmin"`
}

// Display returns the steps as one line each, the way the UI shows them.
func (p Prepared) Display() []string {
	if p.Shell != "" {
		return []string{p.Shell}
	}
	lines := make([]string, len(p.Steps))
	for i, step := range p.Steps {
		lines[i] = strings.Join(step, " ")
	}
	return lines
}

var (
	distroName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	logSize    = regexp.MustCompile(`^[0-9]{1,5}[KMG]$`)
	infName    = regexp.MustCompile(`(?i)^oem[0-9]{1,4}\.inf$`)
)

const irreversible = "Pare 无法撤回这个操作"

func distro(params map[string]string) (string, error) {
	name := params["distro"]
	if !distroName.MatchString(name) {
		return "", fmt.Errorf("发行版名 %q 不合法", name)
	}
	return name, nil
}

// target validates the destination of a WSL move: a new folder at an absolute, unprotected
// location whose parent exists.
func target(params map[string]string) (string, error) {
	path := filepath.Clean(params["target"])
	if !filepath.IsAbs(path) || filepath.VolumeName(path) == "" || strings.ContainsAny(path, "\"\x00\r\n") {
		return "", fmt.Errorf("目标位置 %q 需要是完整的 Windows 路径", params["target"])
	}
	if protected, reason := plan.Protected(path); protected {
		return "", fmt.Errorf("目标位置是受保护的位置：%s", reason)
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("目标位置 %s 已经存在，请换一个新文件夹", path)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
		return "", fmt.Errorf("目标位置的上级文件夹 %s 不存在", filepath.Dir(path))
	}
	return path, nil
}

var builders = map[string]func(map[string]string) (Prepared, error){
	"wsl_move": func(params map[string]string) (Prepared, error) {
		name, err := distro(params)
		if err != nil {
			return Prepared{}, err
		}
		to, err := target(params)
		if err != nil {
			return Prepared{}, err
		}
		return Prepared{
			Steps: [][]string{{"wsl.exe", "--terminate", name}, {"wsl.exe", "--manage", name, "--move", to}},
			Warnings: []string{"会先关闭正在运行的 " + name + "，其中未保存的工作会丢失",
				"搬迁要复制整个虚拟磁盘，可能需要几分钟，期间不要关闭 Pare", irreversible + "，但可以用同样的命令搬回来"},
		}, nil
	},
	"wsl_set_sparse": func(params map[string]string) (Prepared, error) {
		name, err := distro(params)
		if err != nil {
			return Prepared{}, err
		}
		return Prepared{
			Steps:    [][]string{{"wsl.exe", "--terminate", name}, {"wsl.exe", "--manage", name, "--set-sparse", "true"}},
			Warnings: []string{"会先关闭正在运行的 " + name, "部分 WSL 版本出于数据安全考虑可能拒绝转换，失败时会显示原因"},
		}, nil
	},
	"wsl_journal_vacuum": func(params map[string]string) (Prepared, error) {
		name, err := distro(params)
		if err != nil {
			return Prepared{}, err
		}
		size := strings.ToUpper(params["size"])
		if size == "" {
			size = "100M"
		}
		if !logSize.MatchString(size) {
			return Prepared{}, fmt.Errorf("日志大小 %q 不合法，例如 100M", params["size"])
		}
		return Prepared{
			Steps:    [][]string{{"wsl.exe", "-d", name, "-u", "root", "-e", "journalctl", "--vacuum-size=" + size}},
			Warnings: []string{"会删除较早的系统日志", irreversible},
		}, nil
	},
	"wsl_apt_clean": func(params map[string]string) (Prepared, error) {
		name, err := distro(params)
		if err != nil {
			return Prepared{}, err
		}
		return Prepared{Steps: [][]string{{"wsl.exe", "-d", name, "-u", "root", "-e", "apt-get", "clean"}}, Warnings: []string{"apt 之后需要时会重新下载安装包"}}, nil
	},
	"npm_cache_clean": func(map[string]string) (Prepared, error) {
		return Prepared{Steps: [][]string{{"cmd.exe", "/d", "/c", "npm", "cache", "clean", "--force"}}, Warnings: []string{"npm 之后会重新下载需要的包"}}, nil
	},
	"pip_cache_purge": func(map[string]string) (Prepared, error) {
		return Prepared{Steps: [][]string{{"cmd.exe", "/d", "/c", "pip", "cache", "purge"}}, Warnings: []string{"pip 之后会重新下载需要的包"}}, nil
	},
	"docker_prune": func(map[string]string) (Prepared, error) {
		return Prepared{Steps: [][]string{{"docker", "system", "prune", "-f"}}, Warnings: []string{"会删除已停止的容器、未使用的网络、悬空镜像和构建缓存", irreversible}}, nil
	},
	"dism_component_cleanup": func(map[string]string) (Prepared, error) {
		return Prepared{
			Steps:      [][]string{{"Dism.exe", "/Online", "/Cleanup-Image", "/StartComponentCleanup"}},
			Warnings:   []string{"可能需要十几分钟", "清理后已安装的更新可能无法再卸载", irreversible},
			NeedsAdmin: true,
		}, nil
	},
	"driver_delete": func(params map[string]string) (Prepared, error) {
		inf := strings.ToLower(params["inf"])
		if !infName.MatchString(inf) {
			return Prepared{}, fmt.Errorf("驱动包名 %q 不合法，应类似 oem27.inf", params["inf"])
		}
		return Prepared{
			Steps:      [][]string{{"pnputil.exe", "/delete-driver", inf}},
			Warnings:   []string{"正在被设备使用的驱动包会被系统拒绝删除（不会强制）", irreversible},
			NeedsAdmin: true,
		}, nil
	},
	"disable_hibernation": func(map[string]string) (Prepared, error) {
		return Prepared{
			Steps:      [][]string{{"powercfg.exe", "/hibernate", "off"}},
			Warnings:   []string{"会关闭休眠和快速启动，可以用 powercfg /hibernate on 恢复"},
			NeedsAdmin: true,
		}, nil
	},
}

var ErrUnknownTemplate = errors.New("不是受支持的动作模板")

// Templates lists the IDs Prepare understands.
func Templates() []string {
	ids := make([]string, 0, len(builders))
	for id := range builders {
		ids = append(ids, id)
	}
	return ids
}

// Prepare validates a template's parameters and returns the exact steps to run.
func Prepare(template string, params map[string]string) (Prepared, error) {
	build, ok := builders[template]
	if !ok {
		return Prepared{}, fmt.Errorf("%w：%s", ErrUnknownTemplate, template)
	}
	prepared, err := build(params)
	prepared.Template = template
	return prepared, err
}

// PrepareCommand wraps a command the AI wrote outside the templates. Pare cannot check
// what it does; the user must read it and choose to run it.
func PrepareCommand(line string) (Prepared, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.ContainsAny(line, "\r\n\x00") {
		return Prepared{}, errors.New("命令为空或包含换行")
	}
	return Prepared{
		Shell:    line,
		Warnings: []string{"这是 AI 写的命令，不在 Pare 的受支持动作里，Pare 无法检查它会做什么", "请逐字读懂后再决定是否执行", irreversible},
	}, nil
}

// Run executes the steps in order and stops at the first failure.
func Run(ctx context.Context, prepared Prepared) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	if prepared.Shell != "" {
		return winexec.RunShell(runCtx, prepared.Shell)
	}
	var outputs []string
	for _, step := range prepared.Steps {
		output, err := winexec.Run(runCtx, step)
		outputs = append(outputs, "$ "+strings.Join(step, " "), output)
		if err != nil {
			return strings.Join(outputs, "\n"), fmt.Errorf("%s 失败：%w", step[0], err)
		}
	}
	return strings.Join(outputs, "\n"), nil
}
