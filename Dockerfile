FROM golang:bullseye
RUN apk add ceph
WORKDIR /opt/cephapi
COPY . .
RUN go build -mod=vendor -o cephapi main.go
CMD [ "./cephapi" ]