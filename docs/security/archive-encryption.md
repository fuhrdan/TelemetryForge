# Incident Archive Encryption

A `.tfincident` can contain full-fidelity production evidence.

v1.5.0 therefore supports optional authenticated encryption around the complete
ZIP archive.

## Construction

Encrypted archives use:

```text
AES-256-GCM
```

The outer payload is:

```text
TFINENC1 magic
GCM nonce
authenticated ciphertext of complete ZIP
```

Authenticated additional data identifies the TelemetryForge archive encryption
format.

A wrong key or modified ciphertext fails authentication before ZIP parsing.

## Key format

TelemetryForge deliberately does not invent a password KDF in v1.5.

The CLI requires a real 256-bit key file encoded as exactly:

```text
64 hexadecimal characters
```

Example generation:

```bash
openssl rand -hex 32 > incident-archive.key
chmod 600 incident-archive.key
```

Export:

```bash
telemetryctl incident export \
  --id INC-42 \
  --out INC-42.tfincident.enc \
  --encrypt-key-file incident-archive.key
```

Verify:

```bash
telemetryctl incident verify \
  --file INC-42.tfincident.enc \
  --decrypt-key-file incident-archive.key
```

The environment alternative is:

```text
TELEMETRYFORGE_ARCHIVE_KEY_FILE
```

## Why not accept passwords?

Converting human passwords into encryption keys safely requires a deliberate
KDF policy: parameters, memory cost, format negotiation, and future upgrade
rules.

v1.5 avoids a weak home-grown password scheme. Supply a random key from your
approved secret-management process.

## Security properties

Encryption provides confidentiality and integrity of the archive file when the
key remains secret.

It does not:

- anonymize telemetry;
- erase sensitive source data from TelemetryForge storage;
- protect an already generated unencrypted HTML report;
- provide public-key recipient identity;
- replace access control or secure key distribution.

## Repository hygiene

`.tfincident`, `.tfincident.enc`, archive key files, and the local
`incident-exports/` directory are Git-ignored by default.

Do not commit real incident evidence or archive keys.
