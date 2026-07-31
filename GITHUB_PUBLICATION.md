# Publishing this repository

This copy was prepared for public hosting.

Before deployment:

1. Copy `deploy/production.env.example` to `deploy/production.env` and provide real values locally.
2. Copy `deploy/livekit.yaml.example` to `deploy/livekit.yaml` and provide a new LiveKit API key and secret.
3. Never commit either generated file.
4. Install frontend dependencies with `npm ci`; `node_modules` is intentionally excluded.

Any credentials that existed in an earlier archive should be rotated before use.
