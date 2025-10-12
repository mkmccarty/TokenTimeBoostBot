# --------------------------------------------------
# Stage 1: dlv (cached)
# --------------------------------------------------
ARG GO_VERSION=1.25.2
FROM golang:${GO_VERSION}-alpine AS dlv
RUN apk add --no-cache ca-certificates
# Build Delve as a static binary
ENV CGO_ENABLED=0
RUN go install github.com/go-delve/delve/cmd/dlv@v1.22.1 
# or latest release

# --------------------------------------------------
# Stage 2: build app
# --------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates

# Copy dlv from cache layer
COPY --from=dlv /go/bin/dlv /usr/local/bin/dlv

# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with debug flags
RUN CGO_ENABLED=0 GOOS=linux go build -gcflags "all=-N -l" -o /out/bot .

# --------------------------------------------------
# Stage 3: final image
# --------------------------------------------------
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/bot /app/bot
COPY --from=build /usr/local/bin/dlv /app/dlv
COPY emoji /app/emoji

USER nonroot:nonroot
EXPOSE 4000

# Delve as default CMD for debugging
CMD ["/app/dlv", "--listen=:4000", "--headless=true", "--log=true", "--accept-multiclient", "--api-version=2", "exec", "/app/bot"]
