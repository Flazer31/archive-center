# Windows Trust And Defender Notes

Archive Center starts local services on the user's machine:

- Go backend: `0.0.0.0:28080` by default
- MariaDB: `127.0.0.1:3307` by default
- ChromaDB: `127.0.0.1:8000` only in `full_local` / `bundled` vector mode

It does not disable Microsoft Defender, does not add Defender exclusions, and
does not install a persistent Windows service.

## If Defender Or SmartScreen Blocks A File

Do not submit the whole release zip first. Submit the exact detected file when
possible.

1. Open Windows Security > Virus & threat protection > Protection history.
2. Record the detection name, affected file path, and Defender definition
   version if shown.
3. Run this from the package folder to generate local evidence:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check-windows-trust.ps1
```

4. Open `.runtime\reports\windows-trust-report.json` and find the matching
   file path and SHA256.
5. Submit the exact file to Microsoft Security Intelligence:
   `https://www.microsoft.com/en-us/wdsi/filesubmission`

Use these submission choices when they match the situation:

- Submit file as: `Software developer`
- Product: `Microsoft Defender Antivirus` or `Microsoft Defender SmartScreen`
- What do you believe this file is: `Incorrectly detected as malware/malicious`
- Additional information: include the file path, SHA256, package version, and
  local service ports listed above.

## Release-Build Trust Checklist

- Prefer signed Archive Center-owned binaries and PowerShell scripts.
- Keep `PACKAGE_FILE_MANIFEST.json` and `SHA256SUMS.txt` in the package.
- Keep the 2.3 release line as one standard package; use runtime profiles for
  lighter behavior instead of shipping a separate Lite package.
- Avoid hidden shell launch chains when a child process can be started directly.
- Avoid automatic Defender exclusions. Users may add their own local exception,
  but the package must not do that for them.
- Remove `.runtime`, database files, logs, caches, test outputs, and local
  development artifacts before zipping.

## Why A Clean File Can Still Be Blocked

Small-batch unsigned software can be blocked before it has reputation. Bundled
runtime payloads such as database servers or Python packages can also increase
the chance of static or behavior-based suspicion. Signing, stable hashes,
minimal packaged artifacts, and Microsoft false-positive submission are the
proper release path.
