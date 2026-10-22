# Cold Email deployment

Use the root README and `make start` to build this checkout and start its local Compose stack. The only external service key is `PRAMAAN_API_KEY`; generated instance keys are private in `.env`.

The Dockerfiles build local images. There is no automatic image publication, upstream updater or telemetry upload. Web and admin builds use the repository root as their build context so the included Pramaan SDK is available.

Postgres, Redis, NATS, local storage and the live-update service are the app’s own infrastructure. If you run workers on separate machines, configure their authenticated connection to your own backend and supply the Pramaan key privately. Examples in `split-cloud/` describe owned infrastructure, not vendor mailbox credentials.

Production hosting, TLS certificates, backups and publication are owner tasks. This template has not been published or tested against live Pramaan services.
