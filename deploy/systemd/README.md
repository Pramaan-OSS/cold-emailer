# systemd units for a Docker-free install

One unit per ColdEmail process, for a host where the binaries were built from
source and installed under `/opt/coldemail`. The step-by-step guide that uses
them is [Deploying without Docker](https://example.test/development/bare-metal/).

| Unit | Runs | Reads |
|------|------|-------|
| `coldemail-backend.service` | `/opt/coldemail/bin/backend` | `/etc/coldemail/coldemail.env` |
| `coldemail-forms.service` | `/opt/coldemail/bin/forms` | `/etc/coldemail/coldemail.env` |
| `coldemail-consumer.service` | `/opt/coldemail/bin/consumer` | `/etc/coldemail/coldemail.env` |
| `coldemail-tracking.service` | `/opt/coldemail/bin/tracking` | `/etc/coldemail/coldemail.env` |
| `coldemail-realtime.service` | `/opt/coldemail/realtime/bin/realtime start` | `/etc/coldemail/coldemail.env` |
| `coldemail-worker.service` | `/opt/coldemail/bin/worker` | `/etc/coldemail/worker.env` |
| `coldemail-updater.service` | `/opt/coldemail/bin/updater` | `/etc/coldemail/updater.env` |

Every unit runs as the unprivileged `coldemail` user, may write only under
`/var/lib/coldemail` (blob storage), and restarts on failure. Copy them to
`/etc/systemd/system/`, then `systemctl daemon-reload` and
`systemctl enable --now` the ones this host runs. Paths are plain strings, so
`sed` them if you install somewhere other than `/opt/coldemail`.

`deploy/config/env.example` is the template for `coldemail.env`; the worker env
file is what `POST /api/v1/workers/enroll` returns for an enrollment token.

`coldemail-updater.service` is the exception to the unprivileged rule: it runs as
the user who owns the checkout (`deploy` in the unit; change it) and drives
`scripts/upgrade-bare-metal.sh`, which builds unprivileged and then runs
`coldemail-install-release.sh` (installed root-owned at
`/usr/local/sbin/coldemail-install-release`) through sudo. That installer takes no
arguments, touches only fixed paths and refuses symlinks, and is the single
command to allow in sudoers. `updater.env` holds only `UPDATER_TOKEN` (the
backend's `INTERNAL_API_TOKEN`). See
[Updates](https://example.test/development/updates/).
