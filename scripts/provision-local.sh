#!/bin/sh
set -eu
if [ "${PRAMAAN_FAKE:-}" = 1 ]; then
  python scripts/fake-pramaan.py & fixture=$!
  trap 'kill "$fixture" 2>/dev/null || true' EXIT
  python - <<'PY'
import socket,time
end=time.monotonic()+20
while True:
    try:
        with socket.create_connection(('127.0.0.1',59090),timeout=1): break
    except OSError:
        if time.monotonic()>=end: raise SystemExit('The local test service did not start.')
        time.sleep(0.1)
PY
fi
instance-setup
