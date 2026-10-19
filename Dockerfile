# Infrasible Monitoring Relay — multi-stage Linux image
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/infrasible-relay ./cmd/infrasible-relay

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --home /var/lib/infrasible-relay --shell /usr/sbin/nologin infrasible-relay \
  && mkdir -p /etc/infrasible-relay /var/lib/infrasible-relay \
  && chown -R infrasible-relay:infrasible-relay /var/lib/infrasible-relay
COPY --from=build /out/infrasible-relay /usr/bin/infrasible-relay
USER infrasible-relay
EXPOSE 8443
ENTRYPOINT ["/usr/bin/infrasible-relay"]
CMD ["run", "--config=/etc/infrasible-relay/config.json"]
