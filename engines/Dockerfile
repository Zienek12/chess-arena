FROM rust:1.85-bookworm AS build

WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release --locked

FROM debian:bookworm-slim

RUN useradd --system --create-home --uid 10001 engine
COPY --from=build /src/target/release/chess-arena-engine /usr/local/bin/chess-arena-engine
USER engine
ENTRYPOINT ["/usr/local/bin/chess-arena-engine"]
