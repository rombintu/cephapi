build:
	go build -mod=vendor -o cephapi main.go

build-legacy:
	go build -mod=vendor -o cephapi -tags nautilus main.go