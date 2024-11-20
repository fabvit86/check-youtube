FROM golang:1.23 AS build-stage
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /check-youtube ./cmd/check-youtube

FROM gcr.io/distroless/base-debian12 AS release-stage
WORKDIR /
COPY --from=build-stage /check-youtube /check-youtube
USER nonroot
ENTRYPOINT ["/check-youtube"]
