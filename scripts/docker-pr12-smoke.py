"""Disposable CI fixtures only; never publishes images or accesses host databases."""
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request

arch = os.environ['AC_TEST_ARCH']
if arch not in ('amd64', 'arm64'):
    raise RuntimeError('Unknown CI architecture')
override = Path('compose.ci-pr12.json')
services = {name: {'platform': 'linux/' + arch} for name in ('backend', 'migrate', 'mariadb', 'chromadb')}
for name in ('backend', 'migrate'):
    services[name].update(image='archive-center:pr12', pull_policy='never')
override.write_text(json.dumps({'services': services}))
compose = ['docker', 'compose', '-p', 'archive-center-pr12-' + arch, '-f', 'compose.yml', '-f', str(override)]
def dc(*args, capture=False):
    result = subprocess.run(compose + list(args), check=True, text=True, capture_output=capture)
    return result.stdout.strip() if capture else None
def request(path, body=None, port=28080):
    req = urllib.request.Request(f'http://127.0.0.1:{port}' + path,
        data=None if body is None else json.dumps(body).encode(),
        headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(req, timeout=5) as response:
        data = response.read()
    return json.loads(data) if data else None
def ready():
    end = time.monotonic() + 240
    while time.monotonic() < end:
        try:
            value = request('/ready')
            if value.get('ready') and value.get('store_ready') and value.get('vector_ready'):
                return
        except (urllib.error.URLError, TimeoutError):
            pass
        time.sleep(2)
    raise RuntimeError('Compose backend did not become ready')
def sql(statement):
    return dc('exec', '-T', 'mariadb', 'mariadb', '--user=archive_center',
              '--password=archive-center-local-pass', '--skip-column-names', 'archive_center', '-e', statement, capture=True)
collection = '/api/v2/tenants/default_tenant/databases/default_database/collections'
try:
    dc('up', '-d', '--no-build')
    ready()
    assert request('/version')['version'] == '4.7.0'
    sql("CREATE TABLE docker_pr12_probe (id INT PRIMARY KEY, value VARCHAR(64)); INSERT INTO docker_pr12_probe VALUES (1,'synthetic-preserved');")
    cid = request(collection, {'name': 'docker_pr12_probe'}, 8000)['id']
    request(collection + '/' + cid + '/add', {'ids': ['one'], 'documents': ['synthetic-preserved'], 'embeddings': [[0.1, 0.2, 0.3]]}, 8000)
    dc('exec', '-T', 'backend', 'sh', '-c', "printf '%s' synthetic-settings > /data/docker-pr12-probe")
    for phase in ('initial', 'restart', 'recreate'):
        if phase == 'restart':
            dc('restart', 'backend', 'mariadb', 'chromadb')
            ready()
        elif phase == 'recreate':
            dc('down')  # Retain the three named data volumes.
            dc('up', '-d', '--no-build')
            ready()
        assert sql('SELECT value FROM docker_pr12_probe WHERE id=1;') == 'synthetic-preserved'
        data = request(collection + '/' + cid + '/get', {'ids': ['one'], 'include': ['documents', 'embeddings']}, 8000)
        assert data['documents'] == ['synthetic-preserved'] and len(data['embeddings'][0]) == 3
        assert dc('exec', '-T', 'backend', 'cat', '/data/docker-pr12-probe', capture=True) == 'synthetic-settings'
        print('PASS', arch, phase, 'SQL row, vector and backend volume preserved', flush=True)
finally:
    dc('ps', '-a')
    # This unique CI project owns only synthetic disposable volumes.
    dc('down', '-v', '--remove-orphans')
    override.unlink(missing_ok=True)
