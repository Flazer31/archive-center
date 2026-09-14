#!/usr/bin/env python3
"""Native offline exporter: stdlib only; never executes Go or opens the database."""
import datetime
import json
import os
import platform
import re
import sys
from pathlib import Path


def collect(directory):
    report = dict(contract_version="archive-center.diagnostics.v1",
                  generated_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  os=platform.system(), version=os.environ.get("AC_BUILD_VERSION", "unknown"), log_directory=str(directory),
                  collector="native_offline", collection_errors=[], files=[])
    allowed = re.compile(r"^(backend(?:-crash)?|launcher(?:\.out|\.err)?|chromadb\.(?:out|err)|mariadb(?:\.out|\.err|-init)?|schema|install|update(?:-candidate\.out|-candidate\.err)?)\.log(?:\.[123])?$")
    try:
        paths = [p for p in directory.iterdir() if allowed.fullmatch(p.name) and p.is_file() and not p.is_symlink()]
    except OSError as error:
        report["collection_errors"].append(str(error))
        paths = []
    bootstrap = Path.home() / "ArchiveCenter-install.log"
    if bootstrap.is_file() and not bootstrap.is_symlink():
        paths.append(bootstrap)
    for path in sorted(paths):
        item = dict(name=path.name)
        try:
            with path.open("rb") as stream:
                length = stream.seek(0, 2)
                stream.seek(max(0, length - 65536))
                text = stream.read(65536).decode("utf-8", "replace")
            text = re.sub(r"(?i)(Bearer\s+)[^\s\"',;]+", r"\1[REDACTED]", text)
            text = re.sub(r"(?i)((?:api[_-]?key|access[_-]?token|authorization|password|secret)[\"']?\s*[:=]\s*[\"']?)[^\s\"',;&}]+", r"\1[REDACTED]", text)
            text = re.sub(r"(?i)(https?://)[^/\s:@]+:[^/\s@]+@", r"\1[REDACTED]@", text)
            text = re.sub(r"[^\s\"':]+:[^\s\"']+@(?:tcp|unix)\([^)]*\)", "[REDACTED]", text)
            text = re.sub(r"(?:sk-[A-Za-z0-9_-]{8,}|AIza[A-Za-z0-9_-]{20,})", "[REDACTED]", text)
            item.update(text=text, truncated=length > 65536)
        except OSError as error:
            item["error"] = str(error)
        report["files"].append(item)
    return report


if __name__ == "__main__":
    # ASCII JSON remains readable with either a UTF-8 or locale-configured shell.
    json.dump(collect(Path(os.environ["AC_LOG_DIR"])), sys.stdout, ensure_ascii=True)
