BINARY  := unzip-riscos
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: all build dist test clean

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

dist:
	$(foreach P,$(PLATFORMS), \
		$(eval OS   := $(word 1,$(subst /, ,$(P)))) \
		$(eval ARCH := $(word 2,$(subst /, ,$(P)))) \
		$(eval EXT  := $(if $(filter windows,$(OS)),.exe,)) \
		GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 \
			go build -trimpath -ldflags="$(LDFLAGS)" \
			-o $(BINARY)-$(OS)-$(ARCH)$(EXT) . ;)

clean:
	rm -f $(BINARY) $(BINARY)-*-*
