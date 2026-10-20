FROM golang:1.23-bookworm AS build

WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 go build -o /out/arena .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install --no-install-recommends --yes docker.io ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --create-home --uid 10001 arena
COPY --from=build /out/arena /usr/local/bin/arena
USER root
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/arena"]
