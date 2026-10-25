<p align="center">
  <img src="docs/images/banner.svg" alt="Pare — Ein KI-Ermittler für Windows-Datenträger" width="100%">
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.fr.md">Français</a> · <b>Deutsch</b>
</p>

<p align="center">
  <img alt="Plattform" src="https://img.shields.io/badge/platform-Windows%2011-0e7d60?style=flat-square">
  <img alt="Version" src="https://img.shields.io/badge/version-0.3.0--beta-3cc59b?style=flat-square">
  <img alt="Lizenz" src="https://img.shields.io/badge/license-MIT-2b3632?style=flat-square">
  <img alt="Go" src="https://img.shields.io/badge/Go-core-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="Svelte" src="https://img.shields.io/badge/Svelte-UI-FF3E00?style=flat-square&logo=svelte&logoColor=white">
</p>

<p align="center">
  <a href="https://github.com/DRAG0NM/Pare/releases/download/v0.3.0/Pare-0.3.0-win-x64.zip"><img alt="Für Windows herunterladen" src="https://img.shields.io/badge/F%C3%BCr%20Windows%20herunterladen-v0.3.0%20Vorschau-0e7d60?style=for-the-badge&logo=windows&logoColor=white"></a>
</p>
<p align="center"><sub>Portable Zip-Datei · 5 MB · ohne Installer · <a href="https://github.com/DRAG0NM/Pare/releases/tag/v0.3.0">Versionshinweise und Prüfsummen</a></sub></p>

Handelsübliche Cleaner räumen die Caches auf, die jeder hat. Der Speicherplatz, der tatsächlich verschwindet, hängt meist an einem einzelnen Rechner: ein Updater, der immer wieder denselben 300-MB-Installer herunterlädt, eine WSL-Festplatte, die wächst, aber nie schrumpft, eine Einstellung, die in den falschen Abschnitt geschrieben und stillschweigend ignoriert wurde, native Binärdateien für Plattformen, die dieser PC nie ausführen wird.

**Pare schickt einen KI-Ermittler hinter genau diese Art von Verschwendung her.** Er liest den Scan, öffnet die Dateien, die er braucht, gleicht installierte Programme und den Systemzustand ab und erklärt jedes Problem: was es ist, warum es entstanden ist, ob es wiederkommt und was Sie dagegen tun können. Auf dem Datenträger ändert sich nichts, bevor Sie die genauen Elemente oder Befehle gesehen und bestätigt haben; Datei-Bereinigungen landen in einer Quarantäne, die sich rückgängig machen lässt.

<p align="center">
  <img src="docs/images/demo.gif" alt="Scan, KI-Untersuchung, Bestätigung und Zusammenfassung der Bereinigung" width="100%">
</p>
<p align="center"><sub>Bildschirme aus dem integrierten Demomodus aufgenommen; die Daten dienen nur der Veranschaulichung.</sub></p>

## Highlights

- **Untersucht, statt Regeln abzugleichen.** Das Modell arbeitet mit schreibgeschützten Werkzeugen: dem Scan-Index, Dateiköpfen und Text, installierten Programmen, laufenden Prozessen und einem festen Satz von Diagnosen (WSL, Treiberspeicher, Komponentenspeicher, Schattenkopien, Docker, Ruhezustand). Jeder Befund wird mit Belegen geliefert.
- **Ursachen und Vorbeugung.** Jeder Befund gibt an, ob die Verschwendung wieder anwachsen wird. Wenn ja, bietet Pare einen Vorbeugungsschritt an, etwa journald innerhalb von WSL zu begrenzen, den Sie nach der Bereinigung mit einem Klick ausführen können.
- **Standardmäßig umkehrbar.** Dateien und Ordner werden in eine Quarantäne auf demselben Volume verschoben und lassen sich Stapel für Stapel wiederherstellen. Das endgültige Löschen ist ein eigener, ausdrücklicher Schritt.
- **Doppelt geprüft.** Geschützte Speicherorte, Dateiidentität, Änderungen seit dem Scan und offene Handles werden beim Erstellen des Plans und noch einmal unmittelbar vor jedem Verschieben geprüft.
- **Ehrliche Zahlen.** Der freie Speicherplatz vor und nach einer Bereinigung wird gemessen, nicht geschätzt; Speicherplatz, der noch in der Quarantäne belegt ist, wird gesondert ausgewiesen.
- **Schnelle, vollständige Scans.** Parallele Verzeichnisaufzählung, mit Sicherungsberechtigung bei erhöhten Rechten, sodass Ordner, die nur das System lesen darf, mitgezählt statt stillschweigend übersprungen werden.

## Ein genauerer Blick

<table>
  <tr>
    <td width="50%"><img src="docs/images/overview.png" alt="Verzeichnisbaum, verknüpft mit einer Treemap"><br><sub><b>Scan</b> — Verzeichnisbaum und Treemap; nicht lesbare Orte werden aufgelistet, statt als null gezählt zu werden.</sub></td>
    <td width="50%"><img src="docs/images/investigate.png" alt="KI-Befunde mit Ursache, Belegen und Optionen"><br><sub><b>KI-Tiefenanalyse</b> — jeder Befund mit Ursache, Belegen, Nachwachsen und Optionen.</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/images/confirm.png" alt="Bestätigungsdialog mit den genauen Befehlen und Elementen"><br><sub><b>Bestätigen</b> — die genauen Elemente und Befehle, Warnungen und erforderliche Administratorrechte.</sub></td>
    <td width="50%"><img src="docs/images/summary.png" alt="Zusammenfassung der Bereinigung mit gemessenem freiem Speicherplatz und Vorbeugungsschritten"><br><sub><b>Zusammenfassung</b> — gemessener freier Speicherplatz, was noch in der Quarantäne liegt, und Vorbeugung per Klick.</sub></td>
  </tr>
</table>

## Erste Schritte

1. Laden Sie [Pare-0.3.0-win-x64.zip](https://github.com/DRAG0NM/Pare/releases/download/v0.3.0/Pare-0.3.0-win-x64.zip) herunter und entpacken Sie die Datei an einen beliebigen Ort. Die Anwendung ist portabel; es gibt nichts zu installieren. Die einzelne `pare-desktop.exe` und die SHA-256-Prüfsummen finden Sie auf der [Release-Seite](https://github.com/DRAG0NM/Pare/releases/tag/v0.3.0).
2. Klicken Sie mit der rechten Maustaste darauf und wählen Sie **Als Administrator ausführen**, um einen vollständigen Scan zu erhalten. Ohne erhöhte Rechte werden Ordner, die nur SYSTEM lesen kann, als nicht lesbar aufgelistet.
3. Wählen Sie ein Laufwerk. Wenn Sie die Bereinigungsphase zum ersten Mal öffnen, fragt Pare nach einem API-Schlüssel eines OpenAI-kompatiblen Anbieters. DeepSeek ist vorausgewählt; Qwen, Kimi, Zhipu, OpenAI und benutzerdefinierte Endpunkte stehen ebenfalls zur Verfügung.
4. Starten Sie eine Tiefenanalyse, prüfen Sie die Befunde, wählen Sie für jeden eine Option und bestätigen Sie.

**Sprachen.** Die Oberfläche ist auf Vereinfachtem Chinesisch, Traditionellem Chinesisch, Englisch, Japanisch, Französisch und Deutsch verfügbar und lässt sich in den Einstellungen umschalten. Auch die KI verfasst ihre Befunde in der gewählten Sprache.

**Voraussetzungen.** Windows 11 x64 (getestet). Windows 10 mit der [WebView2-Laufzeit](https://developer.microsoft.com/microsoft-edge/webview2/) sollte funktionieren, wurde aber nicht getestet. Die ausführbare Datei ist noch nicht codesigniert, daher kann SmartScreen beim ersten Start um eine Bestätigung bitten.

**Kosten und Dauer.** Ein Durchlauf in Standardtiefe dauerte auf dem Entwicklungsrechner etwa zwei Minuten und verbrauchte 1,3 bis 1,5 Millionen Token. Mit dem Prompt-Caching von DeepSeek kosteten fünf Durchläufe insgesamt ¥0.73, etwa ¥0.15 pro Durchlauf. Tiefere Stufen dauern länger.

## So funktioniert es

```text
 Scan ──► AI deep dive ──► Confirm ──► Execute ──► Summary ──► Quarantine
 read-only   read-only tools   exact steps   quarantine first   measured     restore or
 index       + your consent    you approve   then templates     free space   purge later
```

| Phase | Was geschieht |
|---|---|
| **Scan** | Zählt das Laufwerk oder den Ordner parallel auf und erstellt einen Index aus Größen, Datei-IDs und Zeitstempeln. Reparsepunkte werden aufgelistet, aber nicht verfolgt. |
| **Tiefenanalyse** | Das Modell beginnt mit der Übersicht und lokal berechneten Hinweisen auf Auffälligkeiten (riesige Dateien, virtuelle Festplatten, Gruppen gleich großer Dateien, nebeneinander vorhandene Versionen, Binärdateien für mehrere Plattformen), grenzt dann mit seinen Werkzeugen ein und meldet Befunde, sobald die Belege ausreichen. Es gibt drei Tiefenstufen; ein Folgedurchlauf sucht nach Problemen, die noch nicht gemeldet wurden. |
| **Bestätigen** | Zeigt für jede gewählte Option die Dateien, die verschoben werden, oder die Befehle, die ausgeführt werden, samt Warnungen und dem Hinweis, ob Administratorrechte nötig sind. Befehle außerhalb der unterstützten Vorlagen müssen als gelesen angehakt werden. |
| **Ausführen** | Quarantänen laufen zuerst. Unterstützte Vorlagen werden als validierte Argument-Arrays ausgeführt, nie über eine Shell. |
| **Zusammenfassung** | Freier Speicherplatz vor und nach der Bereinigung, Bytes in Quarantäne mit Löschen-Schaltfläche, Ergebnisse je Option und Vorbeugungsschritte. |
| **Quarantäne** | Jeder Stapel auf lokalen Laufwerken, wiederherstellbar, ohne etwas zu überschreiben, das inzwischen den ursprünglichen Pfad belegt. Ein schlichter Verlauf früherer Bereinigungen steht am Ende. |

**Unterstützte Aktionsvorlagen:** eine WSL-Distribution auf ein anderes Laufwerk verschieben, ihre Festplatte sparse machen, journald verkleinern oder begrenzen und apt innerhalb von WSL bereinigen, npm- und pip-Caches bereinigen, `docker system prune`, DISM-Komponentenbereinigung, ein einzelnes Treiberpaket löschen, den Ruhezustand ausschalten.

## Datenschutz und Sicherheit

- **Was Ihren Rechner verlässt.** Verzeichnisstruktur, Dateinamen und der Inhalt der Dateien, die das Modell zum Lesen auswählt, werden an den von Ihnen konfigurierten Anbieter gesendet. Das Lesen von Dateien, die Zugangsdaten ähneln (SSH-Schlüssel, `.env`, Browser-Anmeldedaten, Zertifikate, Anmeldeinformationsspeicher), erfordert jedes Mal Ihre Zustimmung.
- **Was das Modell tun kann.** Seine Werkzeuge sind schreibgeschützt. Es schlägt vor; Pare validiert; Sie entscheiden. Text innerhalb gescannter Dateien wird als Daten behandelt, nie als Anweisung.
- **Wo Ihr Schlüssel liegt.** Der API-Schlüssel wird mit Windows DPAPI verschlüsselt unter `%APPDATA%\Pare\advisor.json` abgelegt und kann nur von Ihrem Windows-Konto entschlüsselt werden.
- **Was nie angetastet wird.** Das Stammverzeichnis des Volumes, `Windows` (außer den Ordnern für temporäre Dateien, Update-Downloads und Speicherabbilder), `System Volume Information`, der Papierkorb, `Recovery`, die Stammverzeichnisse von Profil und `AppData`, Auslagerungs- und Ruhezustandsdateien, `ProgramData\Microsoft` sowie `Program Files\WindowsApps` werden vom Ausführer selbst abgelehnt, was auch immer das Modell vorschlägt.

## Bekannte Einschränkungen

- Die Befunde schwanken zwischen den Durchläufen. Große Posten wie der WinSxS-Komponentenspeicher oder doppelte Treiberpakete werden nicht jedes Mal gemeldet; ein Folgedurchlauf deckt in der Regel mehr ab.
- Die Einschätzung des Modells, ob eine Anwendung eine Datei noch braucht, ist nicht immer konsistent. Mit „需谨慎“ (Vorsicht) markierte Elemente verdienen einen genaueren Blick, und die Quarantäne hält sie wiederherstellbar.
- Vorlagen und Befehle kann Pare nicht rückgängig machen. Lesen Sie die Bestätigung sorgfältig.
- Die Verzeichnisaufzählung sieht weder NTFS-Metadaten (`$MFT` und Verwandte) noch reservierten Speicher, daher listet ein Scan des gesamten Volumes weniger auf, als belegt ist. Werkzeuge, die die MFT lesen, etwa WizTree, können diese Einträge anzeigen.

## Aus dem Quellcode bauen

```powershell
pnpm --dir desktop/frontend install
pnpm --dir desktop/frontend build
go build -tags desktop,production -ldflags "-w -s -H windowsgui" -o pare-desktop.exe ./desktop
```

Die Wails-CLI wird nicht benötigt; Wails v2 wird als Go-Bibliothek verwendet. `pnpm --dir desktop/frontend dev` stellt die Oberfläche mit integrierten Demodaten im Browser bereit, für die Arbeit am Layout. Das Symbol und die Versionsressourcen der ausführbaren Datei werden aus `desktop/winres` mit [go-winres](https://github.com/tc-hib/go-winres) erzeugt.

Der Kommandozeilenkern (`pare scan`, `plan`, `apply`, `undo`) ist in [docs/cli.md](docs/cli.md) dokumentiert. Designnotizen und die gestufte Roadmap finden Sie in [docs/design.md](docs/design.md) und [docs/development-plan.md](docs/development-plan.md) (auf Chinesisch verfasst).

## Lizenz

Pare wird unter der [MIT-Lizenz](LICENSE) veröffentlicht. Die Lizenzen aller in die ausführbare Datei einkompilierten Komponenten sind in [desktop/THIRD_PARTY_LICENSES.txt](desktop/THIRD_PARTY_LICENSES.txt) zusammengestellt; der Hinweis zu den Abhängigkeiten des Kerns steht in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
