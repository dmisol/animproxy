.MAIN: build

build:
	go build -o animproxy main.go pc.go py.go

run:
	make build
	./animproxy