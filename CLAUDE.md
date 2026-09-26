# Working in mikado

## Git

- Commit straight to `master` unless the user says otherwise. Before creating a branch, ask.

## Trying things out

- Never test against the real server (127.0.0.1:47291) or its data in `~/.local/share/mikado`: it holds the user's real journeys.
- `make demo` serves a fresh set of demo journeys on 127.0.0.1:47295 (data in `.demo/`, wiped every run). Point the CLI at it with `MIKADO_SERVER=http://127.0.0.1:47295 ./bin/mikado …` and the frontend with `make dev-web ADDR=127.0.0.1:47295`. Run it in the background and stop it when done.
- `make screenshots` retakes `docs/screenshots` from the demo journeys. Retake them whenever the UI changes visibly; a `-dirty` version line in the shots is fine.
- The demo data is `src/demo/seed.sh`; extend it when a new feature needs showing, and keep it free of real projects and people.
