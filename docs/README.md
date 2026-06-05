# Engineering Project Knowledge Base

This directory contains the complete technical documentation, architecture designs, and development logs for the chess platform project. It serves as the single source of truth for both the software implementation and the theoretical foundation of the engineering thesis.

## Documentation Structure

To navigate the documentation, you can use the following entry points or open this folder as a Vault in Obsidian to view the interactive graph:

- [[index-architecture]] - System architecture, module interactions, containerization, and communication protocols.
- [[index-theory]] - Chess algorithms, mathematical concepts of bitboards, search techniques, and evaluation functions.
- [[index-logs]] - Verification processes, Perft test logs, and debugging documentation.

## Technical Stack Overview

- **Engine Core:** Rust (Bitboards, Move Generation, Search)
- **Orchestration Layer:** Go (Arena, Docker SDK, Tournament Manager)
- **Presentation Layer:** Next.js & WebAssembly (User Interface, Local Move Verification)
- **Data Store:** PostgreSQL (Telemetry, Match History)
- **Automation:** Jenkins (CI/CD Pipeline)