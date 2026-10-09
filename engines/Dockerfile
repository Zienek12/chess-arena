FROM rust:1.85-bookworm AS build

WORKDIR /src
COPY Cargo.toml .
COPY src ./src
RUN cargo build --release

FROM debian:bookworm-slim

RUN useradd --system --create-home --uid 10001 engine
COPY --from=build /src/target/release/uci_stub /usr/local/bin/uci_stub
USER engine
ENTRYPOINT ["/usr/local/bin/uci_stub"]
