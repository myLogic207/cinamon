GO = go
OUT = bin
DOCKER_USERNAME ?= myLogic207
VERSION ?= 0.2.0

ifeq ($(OS),Windows_NT)     # is Windows_NT on XP, 2000, 7, Vista, 10...
    detected_OS := Windows
else
    detected_OS := $(shell uname)  # same as "uname -s"
endif

.PHONY: all clean copy_active
all: clean cinserve copy_active

.ONESHELL:
SHELL := /bin/sh
%: ./cmd/%
	@echo "Building $<..."
	mkdir -p ${OUT}
	GOOS=linux GOARCH=amd64 ${GO} build -o ./${OUT}/$@_Linux_amd64 ./$<
	GOOS=windows GOARCH=amd64 ${GO} build -o ./${OUT}/$@_Windows_amd64 ./$<
	@echo "Done building $@"
	cp ./${OUT}/$@_$(strip $(detected_OS))_amd64 ./${OUT}/$@

clean:
	rm -rf ${OUT}/

run: cinserve
	./${OUT}/cinserve
