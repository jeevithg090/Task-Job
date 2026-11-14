# syntax=docker/dockerfile:1

# go-sqlite3 uses cgo, so build with a C toolchain and run on a glibc base.
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/backend-auth .

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=build /out/backend-auth /app/backend-auth
ENV PORT=8080 \
    DATABASE_PATH=/data/users.db
VOLUME ["/data"]
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/backend-auth"]
