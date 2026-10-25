# System Architecture and Infrastructure

This section documents the high-level design of the distributed chess platform, focusing on container isolation, process orchestration, and network communication between modules.

## Architectural Components

Select a topic to explore the implementation details:

- [Engine implementation](engine-implementation.md)
- [Tournament model](tournament-model.md)

## Versioned engine runtime

The backend supports two engine sources:

- `local`, which uses `CHESS_ENGINE_PATH` or the repository's debug binary.
- Named Docker images configured with `CHESS_ENGINE_IMAGES`, a comma-separated
  allowlist in the form `version=image`.

The selected version is sent in the WebSocket `state` message and can be
changed by sending `{"type":"reset","engine":"<version>"}`. Docker-backed
versions are started with `docker run --rm -i --init`, so the backend process
needs access to the Docker CLI and daemon. The root `docker-compose.yml`
provides PostgreSQL and a backend service for local platform development.
