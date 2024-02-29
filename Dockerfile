FROM golang:1.21 as backend-builder

WORKDIR /app
ENV CGO_ENABLED=0
ENV GOPROXY=https://proxy.golang.org,direct
COPY server/ .

RUN go build \
  -a \
  -trimpath \
  -ldflags "-s -w -extldflags='-static'" \
  -o server \
  ./main.go

EXPOSE 8080

CMD ["/app/server"]
