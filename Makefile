export GOPATH := $(shell echo $$GOPATH):$(shell pwd)

args	:=-rtmfp=1945,1946 -ncpu=1 -parallel=32 -apps=introduction,askFor \
	-manage=300 -retrans=300,500,1000,1500,1500,2500,3000,4000,5000,7500,10000,15000 \
	-http=6000 -debug -heartbeat=5

all: build-version
	go build -o bin/xserver cmd/main.go
	@cp -f bin/xserver bin/xserver.`git log --date=iso --pretty=format:"%h" -1`
	./bin/xserver ${args}

debug: build-version
	go build -o bin/xserver.prof cmd/prof.go
	./bin/xserver.prof ${args}

build-version:
	@bash genver.sh

clean:
	@rm -rf bin/
	@if [ -d log ]; then cd log && truncate -s 0 *; fi

