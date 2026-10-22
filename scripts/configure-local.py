"""Preserve settings and fill missing private instance keys on first startup."""
from pathlib import Path
import base64
import os
import secrets


def configure(root: Path):
    path = root / '.env'
    if not path.exists():
        path.write_text((root / '.env.example').read_text())
    path.chmod(0o600)
    text = path.read_text()
    values = {}
    for line in text.splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            name, value = line.split('=', 1)
            values[name.strip()] = value.strip().strip('\"\'')
    # Preserve keys from the previous local startup, so stored data stays readable.
    legacy = root / '.data' / 'instance.env'
    old = {}
    if legacy.exists():
        old = dict(line.split('=', 1) for line in legacy.read_text().splitlines() if '=' in line)
    factories = {
        'AUTH_SECRET': lambda: secrets.token_hex(32),
        'SECRET_KEY_BASE': lambda: secrets.token_hex(64),
        'KMS_LOCAL_MASTER_KEY': lambda: base64.b64encode(secrets.token_bytes(32)).decode(),
        'CREDENTIALS_ENCRYPTION_KEY': lambda: secrets.token_hex(32),
        'INTERNAL_API_TOKEN': lambda: secrets.token_hex(32),
        'POSTGRES_PASSWORD': lambda: secrets.token_hex(24),
    }
    for name, factory in factories.items():
        if not values.get(name):
            value = old.get(name) or factory()
            text = '\n'.join(line for line in text.splitlines() if not line.startswith(name + '=')) + '\n' + name + '=' + value + '\n'
    temporary = root / '.env.next'
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w') as stream:
        stream.write(text)
    temporary.replace(path)
    if not values.get('PRAMAAN_API_KEY'):
        print('Add PRAMAAN_API_KEY to .env')


if __name__ == '__main__':
    configure(Path(__file__).resolve().parent.parent)
