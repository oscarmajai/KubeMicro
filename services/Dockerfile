# One image per service: docker build --build-arg SERVICE=catalog -t catalog services/
FROM docker.io/library/golang:1.26-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN test -n "$SERVICE" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/$SERVICE

# Distroless static: no shell, no package manager, runs as UID 65532.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
