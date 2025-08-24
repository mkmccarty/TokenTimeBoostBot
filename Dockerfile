ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go version
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bot .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/bot /app/bot
USER nonroot:nonroot
ENTRYPOINT ["/app/bot"]
