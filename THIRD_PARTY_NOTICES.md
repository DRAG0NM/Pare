# Third-Party Notices

Pare uses `golang.org/x/sys` v0.49.0 for Windows system-call wrappers. The module is maintained by the Go Authors and is distributed under the BSD 3-Clause License. Pare uses its `windows` package for directory enumeration and file metadata.

Source: https://go.googlesource.com/sys/ at tag `v0.49.0`.

```text
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## Desktop app (`pare-desktop.exe`)

The desktop app additionally compiles in the following modules. All are under permissive licenses; their full license texts, as found in the exact versions pinned by `go.sum` and `desktop/frontend/pnpm-lock.yaml`, are collected in [desktop/THIRD_PARTY_LICENSES.txt](desktop/THIRD_PARTY_LICENSES.txt), which must ship with the exe.

| Component | Version | License |
|---|---|---|
| github.com/wailsapp/wails/v2 | v2.17.0 | MIT |
| github.com/wailsapp/go-webview2 | v1.0.22 | MIT |
| github.com/bep/debounce | v1.2.1 | MIT |
| github.com/gabriel-vasile/mimetype | v1.4.16 | MIT |
| github.com/go-ole/go-ole | v1.3.0 | MIT |
| github.com/leaanthony/go-ansi-parser, slicer, u | v1.6.1, v1.6.0, v1.1.1 | MIT |
| github.com/rivo/uniseg | v0.4.7 | MIT |
| github.com/samber/lo | v1.49.1 | MIT |
| git.sr.ht/~jackmordaunt/go-toast/v2 | v2.0.3 | MIT or Unlicense |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause |
| github.com/pkg/browser | v0.0.0-20240102092130 | BSD-2-Clause |
| github.com/pkg/errors | v0.9.1 | BSD-2-Clause |
| github.com/tkrajina/go-reflector | v0.5.8 | Apache-2.0 (no NOTICE file) |
| golang.org/x/net, x/text | v0.56.0, v0.39.0 | BSD-3-Clause |
| svelte (runtime, bundled) | 5.57.2 | MIT |
| clsx, esm-env (svelte runtime dependencies) | 2.1.1, 1.2.2 | MIT |
| d3-hierarchy (bundled) | 3.1.2 | ISC |

The exe also uses the Microsoft Edge WebView2 runtime installed on the system; it is not redistributed with Pare.
