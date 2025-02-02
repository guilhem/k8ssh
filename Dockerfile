FROM golang:1.23 as build

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o /go/bin/app

FROM gcr.io/distroless/static-debian11:nonroot

COPY --from=build /go/bin/app /

ENTRYPOINT ["/app"]
EXPOSE 2222
