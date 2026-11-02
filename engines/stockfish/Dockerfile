# Stockfish for the arena: the analysis engine, and an opponent that can be
# registered like any other engine. Debian's package ships the NNUE network.
FROM debian:trixie-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends stockfish \
    && rm -rf /var/lib/apt/lists/*
USER 65534:65534
ENTRYPOINT ["/usr/games/stockfish"]
