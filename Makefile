SHELL=/usr/bin/env bash

all: build

build: 
	git submodule update --init --recursive
	make -C extern/filecoin-ffi
	go build .

install:
	install -C ./evergreen-dealbot /usr/local/bin/evergreen-dealbot
