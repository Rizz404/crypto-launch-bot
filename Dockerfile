FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ ./cmd/bot ./cmd/datafetch ./cmd/backtest

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ /app/
EXPOSE 8080 9100
ENTRYPOINT ["/app/bot"]
CMD ["run"]
