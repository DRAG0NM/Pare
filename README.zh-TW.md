<p align="center">
  <img src="docs/images/banner.svg" alt="Pare：Windows 上的 AI 磁碟調查員" width="100%">
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <b>繁體中文</b> · <a href="README.ja.md">日本語</a> · <a href="README.fr.md">Français</a> · <a href="README.de.md">Deutsch</a>
</p>

<p align="center">
  <img alt="平台" src="https://img.shields.io/badge/platform-Windows%2011-0c0c0c?style=flat-square&labelColor=1b1b1b">
  <img alt="版本" src="https://img.shields.io/badge/version-0.4.0--beta-3a3a3a?style=flat-square&labelColor=1b1b1b">
  <img alt="授權" src="https://img.shields.io/badge/license-MIT-3a3a3a?style=flat-square&labelColor=1b1b1b">
  <img alt="Go" src="https://img.shields.io/badge/Go-core-0c0c0c?style=flat-square&labelColor=1b1b1b&logo=go&logoColor=white">
  <img alt="Svelte" src="https://img.shields.io/badge/Svelte-UI-0c0c0c?style=flat-square&labelColor=1b1b1b&logo=svelte&logoColor=white">
</p>

<p align="center">
  <a href="https://github.com/DRAG0NM/Pare/releases/download/v0.4.0/Pare-0.4.0-win-x64.zip"><img alt="下載 Windows 版" src="https://img.shields.io/badge/%E4%B8%8B%E8%BC%89%20Windows%20%E7%89%88-v0.4.0%20%E9%A0%90%E8%A6%BD%E7%89%88-d71920?style=for-the-badge&labelColor=0c0c0c&logo=windows&logoColor=white"></a>
</p>
<p align="center"><sub>可攜式壓縮檔 · 5 MB · 免安裝 · <a href="https://github.com/DRAG0NM/Pare/releases/tag/v0.4.0">版本說明與檢查碼</a></sub></p>

一般的清理軟體清的是每台電腦都有的快取。真正讓硬碟空間越來越少的，往往是只屬於這台電腦的問題：更新程式一遍又一遍重複下載同一個 300 MB 的安裝檔，WSL 虛擬磁碟只增不減，設定寫到錯誤的區段而被默默忽略，還有這台電腦永遠不會執行的平台所附帶的原生二進位檔。

**Pare 派出 AI 調查員，專門追查這類浪費。** 它會讀取掃描結果、開啟需要查看的檔案、交叉比對已安裝的程式與系統狀態，並把每個問題說明清楚：是什麼、為什麼會發生、會不會再次出現，以及可以怎麼處理。在您看過確切的項目或指令並確認之前，磁碟上不會有任何變動；檔案類的清理會先移到隔離區，隨時可以還原。

<p align="center">
  <img src="docs/images/demo.gif" alt="掃描、AI 深入調查、確認與清理摘要" width="100%">
</p>
<p align="center"><sub>畫面錄自內建的示範模式，資料僅供示意。</sub></p>

## 主要特色

- **以調查取代規則比對。** 模型使用的工具都是唯讀的：查詢掃描索引、檢視檔案標頭與文字內容、列出已安裝的程式與執行中的處理程序，以及一組固定的診斷項目（WSL、驅動程式存放區、元件存放區、磁碟區陰影複製、Docker、休眠）。每項發現都附有佐證。
- **找出根本原因，也預防再次發生。** 每項發現都會說明這些浪費會不會再長回來。如果會，Pare 會提供預防步驟，例如替 WSL 內的 journald 設定大小上限，清理完成後只要按一下即可執行。
- **預設可還原。** 檔案與資料夾會移到同一個磁碟區的隔離區，可以依批次還原。永久刪除是獨立且需要您明確操作的步驟。
- **雙重檢查。** 受保護的位置、檔案識別資訊、掃描後的異動，以及已開啟的控制代碼，會在建立計畫時檢查一次，並在每次移動前再檢查一次。
- **數字是實測的。** 清理前後的可用空間是實際量測而來，不是估算；仍保留在隔離區的空間則另外列出。
- **掃描快速又完整。** 以平行方式列舉目錄；以系統管理員身分執行時會啟用備份權限，僅限系統存取的資料夾也會被計入，而不是被默默略過。

## 畫面一覽

<table>
  <tr>
    <td width="50%"><img src="docs/images/overview.png" alt="與樹狀圖連動的目錄樹"><br><sub><b>掃描</b>：目錄樹與樹狀圖連動，無法讀取的位置會另外列出，而不是算成零。</sub></td>
    <td width="50%"><img src="docs/images/investigate.png" alt="包含原因、佐證與處理選項的 AI 發現"><br><sub><b>AI 深入調查</b>：每項發現的原因、佐證、是否會再次增長，以及處理選項。</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/images/confirm.png" alt="列出確切指令與項目的確認對話方塊"><br><sub><b>確認</b>：確切的項目與指令、警告，以及是否需要系統管理員權限。</sub></td>
    <td width="50%"><img src="docs/images/summary.png" alt="包含實測可用空間與預防步驟的清理摘要"><br><sub><b>摘要</b>：實測的可用空間、仍留在隔離區的容量，以及一鍵預防。</sub></td>
  </tr>
</table>

## 開始使用

1. 下載 [Pare-0.4.0-win-x64.zip](https://github.com/DRAG0NM/Pare/releases/download/v0.4.0/Pare-0.4.0-win-x64.zip)，解壓縮到任意位置即可。這是可攜式版本，不需要安裝。單獨的 `pare-desktop.exe` 與 SHA-256 檢查碼放在[發行頁面](https://github.com/DRAG0NM/Pare/releases/tag/v0.4.0)。
2. 按右鍵並選擇**以系統管理員身分執行**，才能進行完整掃描。未提升權限時，只有 SYSTEM 能讀取的資料夾會被列為無法讀取。
3. 選擇一個磁碟。第一次進入清理階段時，Pare 會要求您提供相容 OpenAI 的服務供應商的 API 金鑰。預設選用 DeepSeek；也可以選擇 Qwen、Kimi、Zhipu、OpenAI 或自訂端點。
4. 開始深入調查，檢視各項發現，為每一項選擇處理選項，然後確認。

**介面語言。** 介面提供簡體中文、繁體中文、英文、日文、法文與德文六種語言，可在「設定」中切換。AI 也會以所選語言撰寫調查發現。

**系統需求。** Windows 11 x64（已測試）。已安裝 [WebView2 執行階段](https://developer.microsoft.com/microsoft-edge/webview2/)的 Windows 10 應可執行，但尚未測試。執行檔目前尚未進行程式碼簽署，因此第一次啟動時，SmartScreen 可能會要求您確認。

**費用與時間。** 在開發用電腦上，一次標準深度的執行約需兩分鐘，並消耗 130 萬至 150 萬個 token。透過 DeepSeek 的提示快取，五次執行總共花費 ¥0.73，平均每次約 ¥0.15。較深的層級需要更長時間。

## 運作方式

```text
 Scan ──► AI deep dive ──► Confirm ──► Execute ──► Summary ──► Quarantine
 read-only   read-only tools   exact steps   quarantine first   measured     restore or
 index       + your consent    you approve   then templates     free space   purge later
```

| 階段 | 執行內容 |
|---|---|
| **掃描** | 以平行方式列舉磁碟機或資料夾，並建立包含大小、檔案 ID 與時間戳記的索引。重新分析點只會列出，不會跟隨進入。 |
| **深入調查** | 模型從總覽與本機計算出的異常線索（超大檔案、虛擬磁碟、大小相同的檔案群集、並存的多個版本、多平台二進位檔）開始，再用工具逐步縮小範圍，一旦證據足夠就回報發現。提供三種深度；後續執行會尋找尚未回報的問題。 |
| **確認** | 針對每個選擇的選項，顯示將要移動的檔案或將要執行的指令、警告，以及是否需要系統管理員權限。不在支援範本內的指令，必須勾選已讀過才能執行。 |
| **執行** | 先執行隔離。支援的範本會以驗證過的引數陣列執行，不會經過 shell。 |
| **摘要** | 清理前後的可用空間、已隔離的位元組數（附清空按鈕）、各選項的結果，以及預防步驟。 |
| **隔離區** | 列出本機磁碟機上的每個批次，還原時不會覆蓋之後佔用原路徑的任何內容。底部是過往清理的簡單歷史記錄。 |

**支援的動作範本：** 將 WSL 發行版移到其他磁碟機、將其磁碟設為稀疏、壓縮或限制 journald 並清理 WSL 內的 apt、清理 npm 與 pip 快取、`docker system prune`、DISM 元件清理、刪除單一驅動程式套件、關閉休眠。

## 隱私與安全

- **哪些資料會離開您的電腦。** 目錄結構、檔案名稱，以及模型選擇讀取的檔案內容，會傳送給您設定的服務供應商。讀取類似憑證的檔案（SSH 金鑰、`.env`、瀏覽器登入資料、憑證、認證存放區）每次都需要您的同意。
- **模型能做什麼。** 它的工具都是唯讀的。它負責提出建議，Pare 負責驗證，由您決定。被掃描檔案中的文字一律視為資料，絕不當作指令。
- **金鑰存放在哪裡。** API 金鑰以 Windows DPAPI 加密，存放於 `%APPDATA%\Pare\advisor.json`，只有您的 Windows 帳戶能解密。
- **哪些位置絕不會動。** 磁碟區根目錄、`Windows`（其暫存、更新下載與傾印資料夾除外）、`System Volume Information`、資源回收筒、`Recovery`、使用者設定檔與 `AppData` 的根目錄、分頁檔與休眠檔、`ProgramData\Microsoft` 以及 `Program Files\WindowsApps`，無論模型提出什麼建議，執行器本身都會拒絕處理。

## 已知限制

- 每次執行的發現並不完全相同。WinSxS 元件存放區或重複的驅動程式套件這類大型項目，並非每次都會回報；後續執行通常能涵蓋更多。
- 模型對於應用程式是否仍需要某個檔案的判斷，並非始終一致。標示為「需謹慎」（需小心）的項目值得多看一眼，而隔離區能讓它們保持可還原。
- Pare 無法復原範本與指令。請仔細閱讀確認內容。
- 目錄列舉無法看到 NTFS 中繼資料（`$MFT` 等）或保留的儲存空間，因此整個磁碟區的掃描所列出的容量會少於已使用的空間。WizTree 這類讀取 MFT 的工具可以顯示這些項目。

## 從原始碼建置

```powershell
pnpm --dir desktop/frontend install
pnpm --dir desktop/frontend build
go build -tags desktop,production -ldflags "-w -s -H windowsgui" -o pare-desktop.exe ./desktop
```

不需要 Wails CLI；Wails v2 僅作為 Go 程式庫使用。`pnpm --dir desktop/frontend dev` 會在瀏覽器中以內建的示範資料提供介面，方便調整版面。執行檔的圖示與版本資源由 [go-winres](https://github.com/tc-hib/go-winres) 依據 `desktop/winres` 產生。

命令列核心（`pare scan`、`plan`、`apply`、`undo`）的說明請見 [docs/cli.zh-CN.md](docs/cli.zh-CN.md)（簡體中文）。設計說明與分階段路線圖位於 [docs/design.md](docs/design.md) 與 [docs/development-plan.md](docs/development-plan.md)（皆以中文撰寫）。

## 授權

Pare 以 [MIT 授權](LICENSE)發布。編譯進執行檔的所有元件的授權，彙整於 [desktop/THIRD_PARTY_LICENSES.txt](desktop/THIRD_PARTY_LICENSES.txt)；核心的相依套件聲明位於 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
