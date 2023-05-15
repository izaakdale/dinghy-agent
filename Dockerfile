FROM golang:1.20-alpine as builder
WORKDIR /

COPY . .
RUN go mod download

RUN go build -o dinghy-agent .

FROM scratch
WORKDIR /bin
COPY --from=builder /dinghy-agent /bin

CMD [ "/bin/dinghy-agent" ]