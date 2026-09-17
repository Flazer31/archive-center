#!/usr/bin/env bash
# Real fresh-install proof on disposable native CI runners. No model calls.
set -euo pipefail
: "${AC_RELEASE_TAG:?}"
: "${AC_RELEASE_TRANSPORT:?}"
: "${AC_RELEASE_INPUT:?}"
: "${AC_INSTALL_PROOF:?}"
[[ "$AC_RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
case "$AC_RELEASE_TRANSPORT" in candidate|public) ;; *) exit 2 ;; esac
test "${GITHUB_ACTIONS:-}" = true
test -d /run/systemd/system
test ! -e /opt/archive-center
test ! -e /etc/systemd/system/archive-center.service
test ! -e "$HOME/.archive-center"
mkdir -p "$AC_INSTALL_PROOF"
touch "$AC_INSTALL_PROOF/owned-install"
fixture_dir=$(mktemp -d "$RUNNER_TEMP/ac-release-http.XXXXXX")

if [ "$AC_RELEASE_TRANSPORT" = candidate ]; then
  # Only unpublished release HTTP is replaced. Actual entrypoint/helper,
  # package, apt/pip, MariaDB, ChromaDB and systemd execute unchanged.
  python3 - "$AC_RELEASE_INPUT" "$fixture_dir" "$AC_RELEASE_TAG" "$PWD" <<'PY'
from pathlib import Path
import hashlib, json, shutil, sys
assets, fixture, tag, source = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3], Path(sys.argv[4])
packages = list(assets.glob('*.zip'))
assert len(packages) == 1
files = packages + list(assets.glob('SHA256SUMS-*.txt'))
assert len(files) == 2
records = []
for p in files:
    shutil.copyfile(p, fixture / p.name)
    records.append({'name':p.name,'size':p.stat().st_size,'browser_download_url':f'https://github.com/Flazer31/archive-center/releases/download/{tag}/{p.name}','digest':'sha256:'+hashlib.sha256(p.read_bytes()).hexdigest()})
(fixture/'release.json').write_text(json.dumps({'tag_name':tag,'draft':False,'prerelease':False,'assets':records}))
shutil.copyfile(source/'scripts/install-github-release.sh', fixture/'install-github-release.sh')
PY
  cat > "$fixture_dir/curl" <<'PY'
#!/usr/bin/python3
import json, os, subprocess, sys
from pathlib import Path
args = sys.argv[1:]
url = next((a for a in args if a.startswith(('https://','http://'))), '')
root = Path(os.environ['AC_RELEASE_FIXTURE_DIR'])
target = None
if url == 'https://raw.githubusercontent.com/Flazer31/archive-center/main/scripts/install-github-release.sh':
    target = root/'install-github-release.sh'
elif url == 'https://api.github.com/repos/Flazer31/archive-center/releases/latest':
    target = root/'release.json'
else:
    for record in json.loads((root/'release.json').read_text())['assets']:
        if url == record['browser_download_url']:
            target = root/record['name']
if target is None:
    raise SystemExit(subprocess.call(['/usr/bin/curl', *args]))
with (root/'requests.jsonl').open('a') as f:
    f.write(json.dumps({'url':url,'file':target.name})+'\n')
body = target.read_bytes()
if '-o' in args:
    Path(args[args.index('-o')+1]).write_bytes(body)
elif '--output' in args:
    Path(args[args.index('--output')+1]).write_bytes(body)
else:
    sys.stdout.buffer.write(body)
PY
  chmod 755 "$fixture_dir/curl"
  sudo env PATH="$fixture_dir:$PATH" SUDO_USER="$(id -un)" AC_RELEASE_FIXTURE_DIR="$fixture_dir" \
    sh ./install.sh 2>&1 | tee "$AC_INSTALL_PROOF/install.txt"
  sudo cp "$fixture_dir/requests.jsonl" "$AC_INSTALL_PROOF/release-http.jsonl"
else
  # The same normal-user public command, after publication.
  curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh 2>&1 | tee "$AC_INSTALL_PROOF/install.txt"
fi

wait_ready() {
  for attempt in $(seq 1 180); do
    if curl --fail --silent http://127.0.0.1:28080/ready > "$AC_INSTALL_PROOF/ready.json" && \
       curl --fail --silent http://127.0.0.1:28080/version > "$AC_INSTALL_PROOF/version.json"; then
      if python3 - "$AC_INSTALL_PROOF" "${AC_RELEASE_TAG#v}" <<'PY'
from pathlib import Path
import json, sys
d=Path(sys.argv[1]); r=json.loads((d/'ready.json').read_text()); v=json.loads((d/'version.json').read_text())
raise SystemExit(not (r.get('ready') and r.get('store_ready') and r.get('vector_ready') and v.get('version') == sys.argv[2]))
PY
      then return 0; fi
    fi
    sleep 1
  done
  return 1
}
wait_ready
systemctl is-active archive-center.service
systemctl is-enabled archive-center.service
sudo systemctl cat archive-center.service > "$AC_INSTALL_PROOF/service.txt"
python3 - "$AC_INSTALL_PROOF" "${AC_RELEASE_TAG#v}" <<'PY'
from pathlib import Path
import hashlib,json,sys
root=Path('/opt/archive-center/current').resolve()
manifest=json.loads((root/'PACKAGE_FILE_MANIFEST.json').read_text())
assert manifest['package_version']==sys.argv[2]
for entry in manifest['files']:
    b=(root/entry['path']).read_bytes()
    assert len(b)==entry['size_bytes']
    assert hashlib.sha256(b).hexdigest()==entry['sha256'].lower(),entry['path']
assert Path('/opt/archive-center/data-root.txt').read_text().strip()=='/opt/archive-center/data'
assert (root/'migrations/013_precise_memory_text_fields.sql').exists()
result={'version':sys.argv[2],'managed_files':len(manifest['files']),'fresh_service_ready':True,'store_ready':True,'vector_ready':True,'transport':__import__('os').environ['AC_RELEASE_TRANSPORT']}
(Path(sys.argv[1])/'result.json').write_text(json.dumps(result,indent=2))
PY
sudo systemctl restart archive-center.service
wait_ready
printf '%s\n' 'PASS: production fresh entrypoint -> bootstrap -> systemd -> DB/vector ready -> restart; managed file hashes match.'
