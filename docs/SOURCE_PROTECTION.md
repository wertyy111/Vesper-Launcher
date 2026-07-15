# Source and secret protection

Vesper Launcher is a desktop client. Anyone who can run it can copy its files and
inspect its machine code, bundled JavaScript, URLs, and configuration. Obfuscation
raises the cost of reverse engineering; it cannot turn a client-side value into a
secret.

Release rules:

- Never place PATs, Cloudflare Access tokens, service credentials, private keys, or
  reusable shared secrets in source files, frontend variables, or launcher config.
- Keep authorization decisions, privileged API calls, signing keys, and abuse
  controls on the server. Treat every launcher request as attacker-controlled.
- Desktop sessions must be user-specific, revocable, scoped, and short-lived.
- Windows refresh-token persistence uses current-user DPAPI. macOS/Linux builds do
  not persist Microsoft refresh tokens until an OS credential-vault integration is
  available.
- Release builds remove symbols and source maps, obfuscate the launcher assembly,
  scan packaged text files for credentials, and fail when source/debug files remain.
- GitHub Actions dependencies are pinned to immutable commit SHAs. Build jobs have
  read-only repository access; only the verified release job may request write
  access.

If a secret ever appears in a commit, log, artifact, installer, or chat, revoke it
at the provider immediately. Deleting or rewriting the file does not make the old
credential safe again.
