.MAIN: build

build:
	go build -o animproxy main.go pyclient.go dispatcher.go

run:
	make build
	./animproxy