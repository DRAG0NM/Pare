<p align="center">
  <img src="docs/images/banner.svg" alt="Pare — An AI disk investigator for Windows" width="100%">
</p>

<p align="center">
  <b>English</b> · <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows%2011-0e7d60?style=flat-square">
  <img alt="Version" src="https://img.shields.io/badge/version-0.2.0--beta-3cc59b?style=flat-square">
  <img alt="License" src="https://img.shields.io/badge/license-MIT-2b3632?style=flat-square">
  <img alt="Go" src="https://img.shields.io/badge/Go-core-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="Svelte" src="https://img.shields.io/badge/Svelte-UI-FF3E00?style=flat-square&logo=svelte&logoColor=white">
</p>

<p align="center">
  <a href="https://github.com/DRAG0NM/Pare/releases/download/v0.2.0/Pare-0.2.0-win-x64.zip"><img alt="Download for Windows" src="https://img.shields.io/badge/Download%20for%20Windows-v0.2.0%20preview-0e7d60?style=for-the-badge&logo=windows&logoColor=white"></a>
</p>
<p align="center"><sub>Portable zip · 5 MB · no installer · <a href="https://github.com/DRAG0NM/Pare/releases/tag/v0.2.0">release notes and checksums</a></sub></p>

Commercial cleaners clear the caches everyone has. The space that actually goes missing is usually specific to one machine: an updater that downloads the same 300 MB installer over and over, a WSL disk that grows but never shrinks, a setting that was written to the wrong section and silently ignored, native binaries for platforms this PC will never run.

**Pare sends an AI investigator after that kind of waste.** It reads the scan, opens the files it needs, cross-checks installed programs and system state, and explains each problem: what it is, why it happened, whether it will come back, and what you can do about it. Nothing changes on disk until you have seen the exact items or commands and confirmed them, and file cleanups go to a quarantine you can undo.

<p align="center">
  <img src="docs/images/demo.gif" alt="Scan, AI investigation, confirmation and cleanup summary" width="100%">
</p>
<p align="center"><sub>Screens recorded from the built-in demo mode; the data is illustrative.</sub></p>

## Highlights

- **Investigates instead of matching rules.** The model works with read-only tools over the scan index, file headers and text, installed programs, running processes and a fixed set of diagnostics (WSL, driver store, component store, shadow copies, Docker, hibernation). Every finding carries its evidence.
- **Root causes and prevention.** Each finding states whether the waste will grow back. When it will, Pare offers a prevention step, such as capping journald inside WSL, which you can run with one click after the cleanup.
- **Reversible by default.** Files and folders are moved into a same-volume quarantine and can be restored batch by batch. Permanent deletion is a separate, explicit step.
- **Checked twice.** Protected locations, file identity, changes since the scan and open handles are checked when the plan is built and again right before each move.
- **Honest numbers.** Free space before and after a cleanup is measured, not estimated, and space still held in quarantine is reported separately.
- **Fast, complete scans.** Parallel directory enumeration, with backup privilege when elevated so system-only folders are counted instead of silently skipped.

## A closer look

<table>
  <tr>
    <td width="50%"><img src="docs/images/overview.png" alt="Directory tree linked to a treemap"><br><sub><b>Scan</b> — directory tree and treemap, with unreadable locations listed instead of counted as zero.</sub></td>
    <td width="50%"><img src="docs/images/investigate.png" alt="AI findings with cause, evidence and options"><br><sub><b>AI deep dive</b> — each finding with its cause, evidence, regrowth and options.</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/images/confirm.png" alt="Confirmation dialog listing exact commands and items"><br><sub><b>Confirm</b> — the exact items and commands, warnings and admin requirements.</sub></td>
    <td width="50%"><img src="docs/images/summary.png" alt="Cleanup summary with measured free space and prevention steps"><br><sub><b>Summary</b> — measured free space, what is still in quarantine, and one-click prevention.</sub></td>
  </tr>
</table>

## Getting started

1. Download [Pare-0.2.0-win-x64.zip](https://github.com/DRAG0NM/Pare/releases/download/v0.2.0/Pare-0.2.0-win-x64.zip) and extract it anywhere. It is portable; there is nothing to install. The bare `pare-desktop.exe` and SHA-256 checksums are on the [release page](https://github.com/DRAG0NM/Pare/releases/tag/v0.2.0).
2. Right-click it and choose **Run as administrator** for a complete scan. Without elevation, folders that only SYSTEM can read are listed as unreadable.
3. Pick a drive. When you open the clean stage for the first time, Pare asks for an API key from an OpenAI-compatible provider. DeepSeek is preselected; Qwen, Kimi, Zhipu, OpenAI and custom endpoints are also available.
4. Start a deep dive, review the findings, choose an option for each, and confirm.

**Requirements.** Windows 11 x64 (tested). Windows 10 with the [WebView2 runtime](https://developer.microsoft.com/microsoft-edge/webview2/) should work but has not been tested. The executable is not code-signed yet, so SmartScreen may ask you to confirm the first launch.

**Cost and time.** A standard-depth run on the development machine took about two minutes and 1.3–1.5 million tokens. With DeepSeek's prompt caching, five runs cost ¥0.73 in total, about ¥0.15 each. Deeper levels take longer.

## How it works

```text
 Scan ──► AI deep dive ──► Confirm ──► Execute ──► Summary ──► Quarantine
 read-only   read-only tools   exact steps   quarantine first   measured     restore or
 index       + your consent    you approve   then templates     free space   purge later
```

| Stage | What happens |
|---|---|
| **Scan** | Enumerates the drive or folder in parallel and builds an index of sizes, file IDs and timestamps. Reparse points are listed, not followed. |
| **Deep dive** | The model starts from the overview and local anomaly hints (huge files, virtual disks, same-size clusters, side-by-side versions, multi-platform binaries), then narrows down with its tools and reports findings as soon as the evidence is sufficient. Three depths are offered; a follow-up run looks for problems not yet reported. |
| **Confirm** | Shows, for every chosen option, the files that will move or the commands that will run, with warnings and whether administrator rights are needed. Commands outside the supported templates must be ticked as read. |
| **Execute** | Quarantines run first. Supported templates run as validated argument arrays, never through a shell. |
| **Summary** | Free space before and after, quarantined bytes with a purge button, per-option results and prevention steps. |
| **Quarantine** | Every batch on local drives, restorable without overwriting anything that has since taken the original path. A plain history of past cleanups sits at the bottom. |

**Supported action templates:** move a WSL distro to another drive, make its disk sparse, vacuum or cap journald and clean apt inside WSL, clean npm and pip caches, `docker system prune`, DISM component cleanup, delete one driver package, turn hibernation off.

## Privacy and safety

- **What leaves your machine.** Directory structure, file names and the contents of files the model chooses to read are sent to the provider you configure. Reading credential-like files (SSH keys, `.env`, browser login data, certificates, credential stores) requires your consent each time.
- **What the model can do.** Its tools are read-only. It proposes; Pare validates; you decide. Text inside scanned files is treated as data, never as instructions.
- **Where your key lives.** The API key is encrypted with Windows DPAPI under `%APPDATA%\Pare\advisor.json` and can only be decrypted by your Windows account.
- **What is never touched.** The volume root, `Windows` (except its temp, update-download and dump folders), `System Volume Information`, the recycle bin, `Recovery`, profile and `AppData` roots, page and hibernation files, `ProgramData\Microsoft` and `Program Files\WindowsApps` are refused by the executor itself, whatever the model suggests.

## Known limitations

- Findings vary between runs. Large items such as the WinSxS component store or duplicate driver packages are not reported every time; a follow-up run usually covers more.
- The model's judgement of whether an application still needs a file is not always consistent. Items marked "需谨慎" (careful) deserve a closer look, and quarantine keeps them recoverable.
- Templates and commands cannot be undone by Pare. Read the confirmation carefully.
- Directory enumeration cannot see NTFS metadata (`$MFT` and friends) or reserved storage, so a whole-volume scan lists less than the used space. Tools that read the MFT, such as WizTree, can show these entries.
- The interface is in Simplified Chinese.

## Build from source

```powershell
pnpm --dir desktop/frontend install
pnpm --dir desktop/frontend build
go build -tags desktop,production -ldflags "-w -s -H windowsgui" -o pare-desktop.exe ./desktop
```

The Wails CLI is not needed; Wails v2 is used as a Go library. `pnpm --dir desktop/frontend dev` serves the interface in a browser with built-in demo data for layout work. The executable's icon and version resources are generated from `desktop/winres` with [go-winres](https://github.com/tc-hib/go-winres).

The command-line core (`pare scan`, `plan`, `apply`, `undo`) is documented in [docs/cli.md](docs/cli.md). Design notes and the staged roadmap are in [docs/design.md](docs/design.md) and [docs/development-plan.md](docs/development-plan.md) (Chinese).

## License

Pare is released under the [MIT License](LICENSE). Licenses of everything compiled into the executable are collected in [desktop/THIRD_PARTY_LICENSES.txt](desktop/THIRD_PARTY_LICENSES.txt); the core's dependency notice is in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
