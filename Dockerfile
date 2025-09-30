FROM golang:1.24-alpine AS builder
RUN apk add git ca-certificates
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN GO_ENABLED=0 go build

FROM scratch
COPY --from=builder /src/image-pull-creds /bin/image-pull-creds
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/image-pull-creds", "serve"]
