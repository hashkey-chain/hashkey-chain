# Build PlatON in a stock Go builder container
FROM golang:1.16-alpine3.13 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers g++ llvm bash cmake git gmp-dev openssl-dev

ADD . /hashkey-chain
RUN cd /hashkey-chain && make clean && make hskchain

# Pull HashKey-Chain into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates libstdc++ bash tzdata gmp-dev
COPY --from=builder /hashkey-chain/build/bin/hsk-chain /usr/local/bin/

VOLUME /data/hskchain
EXPOSE 6060 6789 6790 6791 16789 16789/udp
CMD ["hsk-chain"]