FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /pulseops ./cmd/pulseops

FROM alpine:3.22
RUN addgroup -g 10001 -S app && adduser -S -D -H -u 10001 -G app app \
    && mkdir /logs && chown app:app /logs
USER app
COPY --from=build /pulseops /usr/local/bin/pulseops
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/pulseops"]
