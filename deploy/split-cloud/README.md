# Split deployment

The supported starting point is the root Compose stack. The files here are examples for running the app’s own bus and worker on machines the owner controls; this split arrangement has not been verified for publication.

The backend keeps Postgres, local encryption and local file storage. A separate worker authenticates to the backend for encrypted keys and uses Pramaan for mailbox operations and remote object storage. It must never receive vendor mailbox credentials. Supply its Pramaan key privately.

Use TLS for connections between owned machines. Keep the internal service tokens private. `control-plane.env.example` and `node/worker.env.example` contain placeholders; they do not create outside accounts or publish images. The owner must configure production hosts, certificates and durable storage before using the split arrangement.
