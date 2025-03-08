FROM golang:1.23 AS build-stage
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN GOOS=linux go build -o /check-youtube ./cmd/check-youtube

FROM gcr.io/distroless/base-debian12 AS release-stage
WORKDIR /
COPY --from=build-stage /check-youtube /check-youtube
ENTRYPOINT ["/check-youtube"]
