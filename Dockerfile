# Production image. Two stages: a builder that carries the whole toolchain, and
# a runtime that carries one static binary.

# --- build ------------------------------------------------------------------
FROM golang:1.26-alpine AS build

ARG TAILWIND_VERSION=v4.3.3

# make drives the generators, and the Makefile asks for bash.
# The musl Tailwind build links libstdc++ and libgcc dynamically,
# and Alpine does not ship them by default.
RUN apk add --no-cache make bash curl libstdc++ libgcc

RUN curl -sSfL -o /usr/local/bin/tailwindcss \
      "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-linux-$(case "$(uname -m)" in aarch64) echo arm64;; *) echo x64;; esac)-musl" \
    && chmod +x /usr/local/bin/tailwindcss

# The Makefile reads this instead of ./bin/tailwindcss.
ENV TAILWIND=/usr/local/bin/tailwindcss

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# .git is outside the build context, so the VCS stamp cannot be read. The image
# records the commit as an OCI label instead. See .github/workflows/deploy.yml.
ENV GOFLAGS=-buildvcs=false

RUN make generate css

RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/pilam ./cmd/pilam

# --- runtime ----------------------------------------------------------------
FROM alpine:3.22

# ca-certificates for outbound TLS. wget comes with busybox and answers the
# container health check below.
RUN apk add --no-cache ca-certificates

RUN adduser -D -H -u 10001 pilam \
    && mkdir -p /var/lib/pilam/media \
    && chown -R pilam:pilam /var/lib/pilam

COPY --from=build /out/pilam /usr/local/bin/pilam

USER pilam
WORKDIR /var/lib/pilam

EXPOSE 8080

# This endpoint also round-trips the database.
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/pilam"]
CMD ["serve"]
