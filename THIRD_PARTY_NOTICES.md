# Third-Party Notices

This file identifies third-party software used by Archive Center. It does not
replace the license text supplied by each upstream project.

Release packaging must preserve upstream `LICENSE`, `LICENCE`, `COPYING`,
`NOTICE`, and equivalent files. When a dependency version changes, this file
and the packaged license inventory must be regenerated and reviewed.

## Bundled runtimes

### MariaDB Community Server 12.3.2

- Project: https://mariadb.org/
- License: GNU General Public License version 2 only
- Corresponding source archive: https://archive.mariadb.org/mariadb-12.3.2/source/
- Archive Center package distribution: not bundled
- Windows installation: downloaded directly from MariaDB's official
  distribution service and installed into the user's separate runtime
  directory after SHA-256 verification

MariaDB is a separate database server process. Archive Center communicates
with it through the MySQL/MariaDB wire protocol. Its `COPYING`, `THIRDPARTY`,
and `CREDITS` files remain inside the separately installed official runtime.

### ChromaDB 1.5.9

- Project: https://github.com/chroma-core/chroma/tree/1.5.9
- License: Apache License 2.0
- Internal full-package license retained at:
  `runtime/ChromaDB/Lib/site-packages/chromadb-1.5.9.dist-info/licenses/LICENSE`
- Windows managed-install license retained under:
  `%LOCALAPPDATA%\ArchiveCenter\runtime\ChromaDB\1.5.9\Lib\site-packages\chromadb-1.5.9.dist-info\licenses`

The bundled or managed ChromaDB environment also contains CPython and Python packages.
Their license texts and notices are retained in the Python runtime and the
respective `*.dist-info/licenses` directories. Those embedded files are
authoritative for the exact runtime build.

The Apache License 2.0 text for `flatbuffers` 25.12.19 and `tokenizers` 0.23.1
is retained at `licenses/Apache-2.0.txt` and in their runtime license directories.

### CPython runtime

- Project: https://www.python.org/
- License: Python Software Foundation License Version 2 and the additional
  historical licenses included with CPython
- Internal full-package license retained at: `runtime/ChromaDB/LICENSE.txt`
- Windows managed-install license retained at:
  `%LOCALAPPDATA%\ArchiveCenter\runtime\Python\3.11.9\LICENSE.txt`

## Go modules used by Archive Center source and release tools

The precise versions are declared in `go-service/go.mod` and
`go-service/go.sum`. The table is an inventory of the source module graph; it
does not mean that every module is linked into every release executable.

The standard Windows package executables
(`archive-center-go`, `archive-center-updater`, and `mariadb-schema`) use the
following external modules: `filippo.io/edwards25519`,
`github.com/go-ole/go-ole`, `github.com/go-sql-driver/mysql`,
`github.com/phpdave11/gofpdi`, `github.com/pkg/errors`,
`github.com/shirou/gopsutil/v4`, `github.com/signintech/gopdf`,
`github.com/yusufpapurcu/wmi`, and `golang.org/x/sys`. Their upstream license
files are supplied by their upstream projects. The PDF-related license texts
are also retained under `licenses/` in the Windows package.
Other entries below are used by tests, transitive source dependencies, or
optional migration tools and may not be present in a standard release binary.

| Module | Version | License |
| --- | --- | --- |
| `github.com/DATA-DOG/go-sqlmock` | v1.5.2 | BSD 3-Clause |
| `github.com/go-sql-driver/mysql` | v1.10.0 | MPL-2.0 |
| `github.com/ledongthuc/pdf` | 5959a4027728 | BSD 3-Clause (tests only) |
| `github.com/phpdave11/gofpdi` | 1f10f9844311 | MIT |
| `github.com/pkg/errors` | v0.8.1 | BSD 2-Clause |
| `github.com/shirou/gopsutil/v4` | v4.26.6 | BSD 3-Clause |
| `github.com/signintech/gopdf` | v0.38.0 | MIT |
| `modernc.org/sqlite` | v1.54.0 | BSD-style 3-Clause |
| `filippo.io/edwards25519` | v1.2.0 | BSD 3-Clause |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT |
| `github.com/ebitengine/purego` | v0.10.2 | Apache-2.0 |
| `github.com/go-ole/go-ole` | v1.3.0 | MIT |
| `github.com/google/pprof` | b9395ee17fa0 | Apache-2.0 |
| `github.com/google/uuid` | v1.6.0 | BSD 3-Clause |
| `github.com/mattn/go-isatty` | v0.0.24 | MIT |
| `github.com/ncruces/go-strftime` | v1.0.0 | MIT |
| `github.com/pmezard/go-difflib` | 5d4384ee4fb2 | BSD 3-Clause |
| `github.com/power-devops/perfstat` | 82ca36839d55 | MIT |
| `github.com/remyoudompheng/bigfft` | 24d4a6f8daec | BSD 3-Clause |
| `github.com/yusufpapurcu/wmi` | v1.2.4 | MIT |
| `golang.org/x/sys` | v0.47.0 | BSD 3-Clause |
| `golang.org/x/tools` | v0.48.0 | BSD 3-Clause |
| `modernc.org/gc/v3` | v3.1.5 | BSD-style 3-Clause |
| `modernc.org/libc` | v1.74.4 | BSD-style 3-Clause |
| `modernc.org/mathutil` | v1.7.1 | BSD-style 3-Clause |
| `modernc.org/memory` | v1.11.0 | BSD-style 3-Clause |

The MPL-2.0 modules remain available in Source Code form at their module
repositories and through the Go module proxy using the exact versions listed
in `go-service/go.sum`. Binary distributions must retain this notice so that
recipients know where to obtain the MPL-covered Source Code.

## Embedded PDF memory transport asset

The opt-in PDF memory transport embeds the unmodified Google Fonts
`NotoSansKR[wght].ttf` variable font so generated Korean and Hanja text remains
searchable and copyable.

- Project: https://fonts.google.com/noto/specimen/Noto+Sans+KR
- Upstream: https://github.com/google/fonts/tree/main/ofl/notosanskr
- Copyright: 2014-2021 Adobe, with Reserved Font Name "Source"
- License: SIL Open Font License 1.1
- Retained license text: `licenses/NotoSansKR-OFL-1.1.txt`

The font is used only to render the already-selected long-term-memory text in
the generated PDF. Archive Center does not modify or rename the font.
