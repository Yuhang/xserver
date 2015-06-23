sources := $(wildcard *.proto)

all: $(patsubst %.proto,%.pb.go,$(sources))

%.pb.go: %.proto
	protoc --go_out=../ $^

clean:
	rm -f ../*.pb.go

.DEFAULT_GOAL := all

