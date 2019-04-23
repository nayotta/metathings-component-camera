DOCKER_EXE=$(shell which docker)
CUR_PATH=$(shell pwd)

protos_from_docker:
	$(DOCKER_EXE) run --rm -v $(CUR_PATH):/go/src/github.com/nayotta/metathings-component-camera nayotta/metathings-development /usr/bin/make -C /go/src/github.com/nayotta/metathings-component-camera/proto
