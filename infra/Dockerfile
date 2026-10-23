FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -mod=readonly -trimpath -o /app ./cmd/${SERVICE}
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -mod=readonly -trimpath -o /healthcheck ./cmd/healthcheck
FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0
RUN apk add --no-cache ca-certificates tzdata && addgroup -S app && adduser -S app -G app
COPY --from=build /app /app
COPY --from=build /healthcheck /healthcheck
USER app
EXPOSE 8080
ENTRYPOINT ["/app"]
