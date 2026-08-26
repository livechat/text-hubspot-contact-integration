FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/contact-sync ./cmd/contact-sync

FROM alpine:3.24

RUN apk add --no-cache ca-certificates
COPY --from=build /out/contact-sync /usr/local/bin/contact-sync

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/contact-sync"]
