export GOPATH := $(shell echo $$GOPATH):$(shell pwd)

args	:=-rtmfp=1945,1946 -ncpu=1 -parallel=32 -apps=introduction,askFor \
	-manage=300 -retrans=300,500,1000,1500,1500,2500,3000,4000,5000,7500,10000,15000 \
	-http=6000 -debug -heartbeat=5

all: build-version
	go build -o xserver cmd/main.go
	@cp -f xserver xserver.`git log --date=iso --pretty=format:"%h" -1`
	./xserver ${args}

debug: build-version
	go build -o xserver.prof cmd/prof.go
	./xserver.prof ${args}

build-version:
	@bash genver.sh

clean:
	@rm -f xserver xserver.*
	@cd log && find -type f -exec truncate -s 0 {} \;

