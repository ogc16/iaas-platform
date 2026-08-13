# syntax=docker/dockerfile:1

# --- build stage -------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
# The migrate binary is included for staged rollouts: run it before deploying
# the new server binary when you want migrations applied as a separate job.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- runtime stage ----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /out/migrate /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
