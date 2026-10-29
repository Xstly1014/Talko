FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=community
RUN CGO_ENABLED=0 go build -trimpath -o /service ./cmd/$SERVICE
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /service /service
ENTRYPOINT ["/service"]
