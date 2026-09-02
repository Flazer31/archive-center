# Archive Center 4.1.0

Archive Center 4.1.0 improves request lifecycle stability and adds opt-in PDF
memory delivery without replacing the existing Text path.

## Main changes

- Reuses the same prepared Archive request when RisuAI retries the same logical
  model request, avoiding duplicate preparation, memory injection, and save work.
- Identifies rerolls and edited regenerations by the stable RisuAI user-message
  row so they replace the existing logical turn; a genuinely new row still adds
  a new turn even when its text is identical.
- Keeps the Windows managed stack running when Ctrl+C shutdown is canceled with
  `N`, and stops it only after explicit `Y` confirmation.
- Adds optional PDF representation for the already-selected long-term-memory
  lane: direct Google `inlineData`, LLM Gateway file blocks, and the experimental
  Yumi Provider Manager `<pm-pdf>` bridge. Text mode remains the default.
- Lets Archive-owned Publisher and continuity reads use the preserved original
  response from Yumi Translator 1.4.2 while leaving the displayed translation
  and outgoing RisuAI payload unchanged.
- Fresh-install helpers now verify the exact selected release ZIP against the
  published SHA-256 list before extraction.

## Install

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex
```

Linux, macOS, and Termux:

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

Windows users who download the ZIP directly can extract it and run
`01_start_archive_center_windows.bat`.

## Update and compatibility

- The in-app update button sends one update request; the backend chooses and
  verifies the correct package, then the managed launcher applies it.
- Managed program files may be added, replaced, or removed. Local environment
  files, MariaDB data, Chroma data, runtime directories, and secrets are not
  managed package files and remain preserved.
- Fresh and upgraded installations apply the same complete, ordered migration
  inventory. Later migrations are not copied back into `001_schema.sql`.
- On candidate startup or readiness failure, managed program files are restored.
  Database rollback is intentionally not attempted, so migrations remain
  expand-first and compatible with the previous backend.
- Direct automatic update is supported from published 3.9.9 through 4.0.9.
  Version 3.9.0 requires the documented data-preserving fresh-install procedure.

## Provider status

- Yumi Provider Manager PDF conversion and model output were observed in RisuAI.
- Direct Vertex Gemini 3.1 Pro transport was observed entering the body
  interceptor, applying one Google PDF representation, and removing the matching
  long-term-memory Text block.
- Google AI Studio and LLM Gateway final-body counts and provider-side recall or
  usage comparisons remain unmeasured; this release does not claim token savings.

## Platforms

Release packages are provided for Windows x64, Linux x64, Linux arm64, macOS
Intel, macOS Apple Silicon, and Termux arm64. Native Windows arm64 is not a
declared 4.1.0 release target.

Checksums are published in `SHA256SUMS-4.1.0.txt`.
