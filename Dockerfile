FROM golang:alpine as builder

COPY ./ /go/src/github.com/dahendel/cattle-herder

RUN apk add --update --no-cache glide git && \
  cd /go/src/github.com/dahendel/cattle-herder && \
  glide install && \
  go build

FROM docker.ena.net:5000/alpine/deploy

COPY --from=builder /go/src/github.com/dahendel/cattle-herder/cattle-herder /usr/local/bin/

CMD /usr/local/bin/cattle-herder