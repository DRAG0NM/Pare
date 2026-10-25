<p align="center">
  <img src="docs/images/banner.svg" alt="Pare — Windows 向け AI ディスク調査員" width="100%">
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <b>日本語</b> · <a href="README.fr.md">Français</a> · <a href="README.de.md">Deutsch</a>
</p>

<p align="center">
  <img alt="プラットフォーム" src="https://img.shields.io/badge/platform-Windows%2011-0c0c0c?style=flat-square&labelColor=1b1b1b">
  <img alt="バージョン" src="https://img.shields.io/badge/version-0.3.0--beta-3a3a3a?style=flat-square&labelColor=1b1b1b">
  <img alt="ライセンス" src="https://img.shields.io/badge/license-MIT-3a3a3a?style=flat-square&labelColor=1b1b1b">
  <img alt="Go" src="https://img.shields.io/badge/Go-core-0c0c0c?style=flat-square&labelColor=1b1b1b&logo=go&logoColor=white">
  <img alt="Svelte" src="https://img.shields.io/badge/Svelte-UI-0c0c0c?style=flat-square&labelColor=1b1b1b&logo=svelte&logoColor=white">
</p>

<p align="center">
  <a href="https://github.com/DRAG0NM/Pare/releases/download/v0.3.0/Pare-0.3.0-win-x64.zip"><img alt="Windows 版をダウンロード" src="https://img.shields.io/badge/Windows%20%E7%89%88%E3%82%92%E3%83%80%E3%82%A6%E3%83%B3%E3%83%AD%E3%83%BC%E3%83%89-v0.3.0%20%E3%83%97%E3%83%AC%E3%83%93%E3%83%A5%E3%83%BC-d71920?style=for-the-badge&labelColor=0c0c0c&logo=windows&logoColor=white"></a>
</p>
<p align="center"><sub>ポータブル zip · 5 MB · インストーラー不要 · <a href="https://github.com/DRAG0NM/Pare/releases/tag/v0.3.0">リリースノートとチェックサム</a></sub></p>

市販のクリーナーが消すのは、どの PC にもあるキャッシュです。実際に消えていく容量は、たいてい 1 台の PC に固有のものです。同じ 300 MB のインストーラーを何度もダウンロードし続けるアップデーター、増える一方で縮まない WSL のディスク、誤ったセクションに書かれて黙って無視されている設定、この PC では決して動かないプラットフォーム向けのネイティブバイナリなどです。

**Pare は、そうした無駄の調査に AI の調査員を送り込みます。** スキャン結果を読み、必要なファイルを開き、インストール済みのプログラムやシステムの状態と照合して、それぞれの問題を説明します。何であるか、なぜ起きたか、再発するか、どう対処できるかです。具体的な項目やコマンドを確認して承認するまで、ディスクには何も変更を加えません。ファイルのクリーンアップは、元に戻せる隔離フォルダーへ移動されます。

<p align="center">
  <img src="docs/images/demo.gif" alt="スキャン、AI による調査、確認、クリーンアップの概要" width="100%">
</p>
<p align="center"><sub>画面は内蔵のデモモードで録画したものです。データは説明用です。</sub></p>

## 特長

- **ルール照合ではなく、調査します。** モデルが使うのは読み取り専用のツールです。スキャンインデックス、ファイルヘッダーとテキスト、インストール済みプログラム、実行中のプロセス、そして固定の診断項目（WSL、ドライバーストア、コンポーネントストア、シャドウコピー、Docker、休止状態）を参照できます。すべての発見には根拠が付きます。
- **根本原因と再発防止。** それぞれの発見について、その無駄が再び増えるかどうかを示します。増える場合、Pare は再発防止の手順を提案します。たとえば WSL 内の journald に上限を設ける手順で、クリーンアップの後にワンクリックで実行できます。
- **既定で元に戻せます。** ファイルとフォルダーは同じボリューム内の隔離フォルダーへ移動され、バッチ単位で復元できます。完全削除は、明示的に行う別のステップです。
- **二重のチェック。** 保護された場所、ファイルの同一性、スキャン以降の変更、開いているハンドルは、プランの作成時と、各項目を移動する直前の 2 回確認されます。
- **数字は実測です。** クリーンアップ前後の空き容量は推定ではなく実測値で、隔離フォルダーにまだ残っている容量は別に報告されます。
- **高速で漏れのないスキャン。** ディレクトリを並列に列挙します。管理者権限で実行した場合はバックアップ権限を使うため、システム専用のフォルダーも読み飛ばされず、集計に含まれます。

## 画面で見る

<table>
  <tr>
    <td width="50%"><img src="docs/images/overview.png" alt="ツリーマップと連動するディレクトリツリー"><br><sub><b>スキャン</b> — ディレクトリツリーとツリーマップ。読み取れない場所は 0 として数えず、一覧に表示します。</sub></td>
    <td width="50%"><img src="docs/images/investigate.png" alt="原因、根拠、選択肢を示す AI の発見"><br><sub><b>AI による詳細調査</b> — 各発見の原因、根拠、再増加の有無、選択肢。</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/images/confirm.png" alt="実行するコマンドと項目を正確に示す確認ダイアログ"><br><sub><b>確認</b> — 正確な項目とコマンド、警告、管理者権限が必要かどうか。</sub></td>
    <td width="50%"><img src="docs/images/summary.png" alt="実測の空き容量と再発防止の手順を示すクリーンアップの概要"><br><sub><b>概要</b> — 実測の空き容量、隔離フォルダーに残っている容量、ワンクリックの再発防止。</sub></td>
  </tr>
</table>

## はじめに

1. [Pare-0.3.0-win-x64.zip](https://github.com/DRAG0NM/Pare/releases/download/v0.3.0/Pare-0.3.0-win-x64.zip) をダウンロードし、好きな場所に展開します。ポータブル版なので、インストールは不要です。`pare-desktop.exe` 単体と SHA-256 チェックサムは[リリースページ](https://github.com/DRAG0NM/Pare/releases/tag/v0.3.0)にあります。
2. 完全なスキャンを行うには、右クリックして**管理者として実行**を選びます。昇格しない場合、SYSTEM だけが読み取れるフォルダーは読み取り不可として表示されます。
3. ドライブを選びます。クリーンアップの段階を初めて開くと、Pare は OpenAI 互換のプロバイダーの API キーを求めます。既定では DeepSeek が選ばれており、Qwen、Kimi、Zhipu、OpenAI、カスタムエンドポイントも選べます。
4. 詳細調査を開始し、発見を確認して、それぞれに対する選択肢を選び、確定します。

**表示言語。** インターフェースは、簡体字中国語、繁体字中国語、英語、日本語、フランス語、ドイツ語の 6 言語で利用でき、設定で切り替えられます。AI も、選択した言語で調査結果を書きます。

**動作環境。** Windows 11 x64（動作確認済み）。[WebView2 ランタイム](https://developer.microsoft.com/microsoft-edge/webview2/)を入れた Windows 10 でも動作するはずですが、未検証です。実行ファイルはまだコード署名されていないため、初回起動時に SmartScreen が確認を求めることがあります。

**費用と所要時間。** 開発用マシンでの標準の深さの実行は、約 2 分で、130 万〜150 万トークンを消費しました。DeepSeek のプロンプトキャッシュにより、5 回の実行で合計 ¥0.73、1 回あたり約 ¥0.15 でした。より深いレベルでは時間がかかります。

## 仕組み

```text
 Scan ──► AI deep dive ──► Confirm ──► Execute ──► Summary ──► Quarantine
 read-only   read-only tools   exact steps   quarantine first   measured     restore or
 index       + your consent    you approve   then templates     free space   purge later
```

| 段階 | 内容 |
|---|---|
| **スキャン** | ドライブまたはフォルダーを並列に列挙し、サイズ、ファイル ID、タイムスタンプのインデックスを作成します。リパースポイントは一覧に載せるだけで、たどりません。 |
| **詳細調査** | モデルは、概要とローカルで算出した異常のヒント（巨大なファイル、仮想ディスク、同じサイズのファイルの集まり、並存するバージョン、複数プラットフォーム向けのバイナリ）から始め、ツールで絞り込み、根拠が十分になった時点で発見を報告します。深さは 3 段階から選べます。続けて実行すると、まだ報告されていない問題を探します。 |
| **確認** | 選んだ各選択肢について、移動されるファイルや実行されるコマンド、警告、管理者権限が必要かどうかを表示します。サポート対象のテンプレート以外のコマンドは、内容を読んだことにチェックを入れる必要があります。 |
| **実行** | 隔離が先に実行されます。サポート対象のテンプレートは、検証済みの引数配列として実行され、シェルは経由しません。 |
| **概要** | クリーンアップ前後の空き容量、隔離中のバイト数（完全削除ボタン付き）、選択肢ごとの結果、再発防止の手順。 |
| **隔離フォルダー** | ローカルドライブ上のすべてのバッチ。復元時は、その後に元のパスを使い始めたものを上書きしません。下部には、過去のクリーンアップのシンプルな履歴があります。 |

**サポート対象のアクションテンプレート：** WSL ディストリビューションを別のドライブへ移動、そのディスクをスパース化、WSL 内の journald の圧縮または上限設定と apt のクリーンアップ、npm と pip のキャッシュのクリーンアップ、`docker system prune`、DISM によるコンポーネントのクリーンアップ、ドライバーパッケージ 1 件の削除、休止状態の無効化。

## プライバシーと安全性

- **外部へ送信されるもの。** ディレクトリ構造、ファイル名、モデルが読むことを選んだファイルの内容は、設定したプロバイダーへ送信されます。資格情報に類するファイル（SSH キー、`.env`、ブラウザーのログインデータ、証明書、資格情報ストア）の読み取りには、そのたびにあなたの同意が必要です。
- **モデルにできること。** ツールはすべて読み取り専用です。モデルが提案し、Pare が検証し、あなたが決めます。スキャン対象のファイル内のテキストはデータとして扱い、指示としては扱いません。
- **キーの保管場所。** API キーは Windows DPAPI で暗号化して `%APPDATA%\Pare\advisor.json` に保存され、あなたの Windows アカウントだけが復号できます。
- **決して触れない場所。** ボリュームのルート、`Windows`（その一時フォルダー、更新ダウンロード、ダンプのフォルダーを除く）、`System Volume Information`、ごみ箱、`Recovery`、プロファイルと `AppData` のルート、ページファイルと休止状態ファイル、`ProgramData\Microsoft`、`Program Files\WindowsApps` は、モデルが何を提案しても、実行側が拒否します。

## 既知の制限

- 発見の内容は実行のたびに異なります。WinSxS のコンポーネントストアや重複したドライバーパッケージのような大きな項目は、毎回報告されるとは限りません。続けて実行すると、通常はより多くをカバーできます。
- アプリケーションがそのファイルをまだ必要としているかどうかについてのモデルの判断は、常に一貫しているわけではありません。「需谨慎」（要注意）と表示された項目は、よく確認する価値があります。隔離機能により、それらは復元可能な状態に保たれます。
- テンプレートとコマンドは、Pare では元に戻せません。確認画面をよくお読みください。
- ディレクトリの列挙では、NTFS のメタデータ（`$MFT` など）や予約済み記憶域が見えないため、ボリューム全体のスキャンで表示される容量は、使用中の容量より少なくなります。WizTree のように MFT を読むツールでは、これらのエントリも表示できます。

## ソースからのビルド

```powershell
pnpm --dir desktop/frontend install
pnpm --dir desktop/frontend build
go build -tags desktop,production -ldflags "-w -s -H windowsgui" -o pare-desktop.exe ./desktop
```

Wails CLI は不要です。Wails v2 は Go のライブラリとして使っています。`pnpm --dir desktop/frontend dev` を実行すると、レイアウト作業用に、内蔵のデモデータを使ったインターフェースをブラウザーで表示できます。実行ファイルのアイコンとバージョンリソースは、[go-winres](https://github.com/tc-hib/go-winres) で `desktop/winres` から生成されます。

コマンドラインのコア（`pare scan`、`plan`、`apply`、`undo`）は [docs/cli.md](docs/cli.md) で説明しています。設計ノートと段階的なロードマップは [docs/design.md](docs/design.md) と [docs/development-plan.md](docs/development-plan.md) にあります（どちらも中国語で書かれています）。

## ライセンス

Pare は [MIT ライセンス](LICENSE)で公開されています。実行ファイルにコンパイルされているすべてのもののライセンスは [desktop/THIRD_PARTY_LICENSES.txt](desktop/THIRD_PARTY_LICENSES.txt) にまとめてあり、コアの依存関係に関する告知は [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) にあります。
