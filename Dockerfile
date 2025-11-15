# syntax=docker/dockerfile:1.7
FROM golang:1.22-alpine3.20 AS build
WORKDIR /src
ENV GOTOOLCHAIN=auto CGO_ENABLED=0 GOFLAGS="-trimpath"

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /out/pr-reviewer ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS final
ENV APP_PORT=:8080
ENV DB_DSN=postgres://app:app@db:5432/app?sslmode=disable
COPY --from=build /out/pr-reviewer /pr-reviewer
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/pr-reviewer"]