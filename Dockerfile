FROM golang:1.24 AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY *.go Makefile .git ./
RUN make build

FROM gcr.io/distroless/static-debian11:nonroot
COPY --from=builder /app/prom-scrape-proxy /prom-scrape-proxy
ENTRYPOINT ["/prom-scrape-proxy"]
CMD ["--help"]