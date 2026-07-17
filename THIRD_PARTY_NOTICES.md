# Third-Party Notices

This file records the third-party software identified in the Archive Center
source tree and the Archive Center 3.0.1 Windows full-package audit. It does
not replace the license text supplied by each upstream project.

Release packaging must preserve upstream `LICENSE`, `LICENCE`, `COPYING`,
`NOTICE`, and equivalent files. When a dependency version changes, this file
and the packaged license inventory must be regenerated and reviewed.

## Bundled runtimes

### MariaDB Community Server 11.4.10

- Project: https://mariadb.org/
- License: GNU General Public License version 2 only
- Corresponding source archive: https://archive.mariadb.org/mariadb-11.4.10/source/
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
- License retained at:
  `runtime/ChromaDB/Lib/site-packages/chromadb-1.5.9.dist-info/licenses/LICENSE`

The bundled ChromaDB environment also contains CPython and Python packages.
Their license texts and notices are retained in the Python runtime and the
respective `*.dist-info/licenses` directories. The audited 3.0.1 Windows
runtime contained 79 `*.dist-info` package records and 109 package-level
license or notice files. Those embedded files are authoritative for the exact
runtime build.

The audited upstream runtime omitted package-local license files for
`flatbuffers` 25.12.19 and `tokenizers` 0.23.1 even though their installed
metadata identifies the Apache License 2.0. Archive Center retains the
unmodified Apache License 2.0 text at `licenses/Apache-2.0.txt`. The Windows
package builder copies that text into both exact `*.dist-info/licenses`
directories and stops the build if either dependency metadata directory or
the license text is missing.

### CPython runtime

- Project: https://www.python.org/
- License: Python Software Foundation License Version 2 and the additional
  historical licenses included with CPython
- License retained at: `runtime/ChromaDB/LICENSE.txt`

## Go modules used by Archive Center source and release tools

The precise versions are declared in `go-service/go.mod` and
`go-service/go.sum`. The table is an inventory of the source module graph; it
does not mean that every module is linked into every release executable.

The standard Windows package executables audited for this release
(`archive-center-go`, `archive-center-updater`, and `mariadb-schema`) use the
following external modules: `filippo.io/edwards25519`,
`github.com/go-ole/go-ole`, `github.com/go-sql-driver/mysql`,
`github.com/shirou/gopsutil/v3`, `github.com/yusufpapurcu/wmi`, and
`golang.org/x/sys`. Their upstream license files were present in the Go module
cache used for the audit. Other entries below are used by tests, transitive
source dependencies, or optional migration tools and may not be present in a
standard release binary.

| Module | Version | License |
| --- | --- | --- |
| `github.com/DATA-DOG/go-sqlmock` | v1.5.2 | BSD 3-Clause |
| `github.com/go-sql-driver/mysql` | v1.8.1 | MPL-2.0 |
| `github.com/shirou/gopsutil/v3` | v3.23.12 | BSD 3-Clause |
| `modernc.org/sqlite` | v1.29.10 | BSD-style 3-Clause |
| `filippo.io/edwards25519` | v1.1.0 | BSD 3-Clause |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT |
| `github.com/go-ole/go-ole` | v1.2.6 | MIT |
| `github.com/google/pprof` | d1b30febd7db | Apache-2.0 |
| `github.com/google/uuid` | v1.6.0 | BSD 3-Clause |
| `github.com/hashicorp/golang-lru/v2` | v2.0.7 | MPL-2.0 |
| `github.com/mattn/go-isatty` | v0.0.20 | MIT |
| `github.com/ncruces/go-strftime` | v0.1.9 | MIT |
| `github.com/pmezard/go-difflib` | 5d4384ee4fb2 | BSD 3-Clause |
| `github.com/power-devops/perfstat` | 5aafc221ea8c | MIT |
| `github.com/remyoudompheng/bigfft` | 24d4a6f8daec | BSD 3-Clause |
| `github.com/yusufpapurcu/wmi` | v1.2.3 | MIT |
| `golang.org/x/sync` | v0.19.0 | BSD 3-Clause |
| `golang.org/x/sys` | v0.40.0 | BSD 3-Clause |
| `golang.org/x/tools` | v0.39.0 | BSD 3-Clause |
| `modernc.org/gc/v3` | 573471604cb6 | BSD-style 3-Clause |
| `modernc.org/libc` | v1.49.3 | BSD-style 3-Clause |
| `modernc.org/mathutil` | v1.6.0 | BSD-style 3-Clause |
| `modernc.org/memory` | v1.8.0 | BSD-style 3-Clause |
| `modernc.org/strutil` | v1.2.0 | BSD-style 3-Clause |
| `modernc.org/token` | v1.1.0 | BSD-style 3-Clause |

The MPL-2.0 modules remain available in Source Code form at their module
repositories and through the Go module proxy using the exact versions listed
in `go-service/go.sum`. Binary distributions must retain this notice so that
recipients know where to obtain the MPL-covered Source Code.
