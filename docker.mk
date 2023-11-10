# docker makefile, included from Makefile, will build/push images with docker or podman

ifeq ($(IMAGETAG),)
IMAGETAG="v$(MAJOR).$(MINOR).$(PATCH)$(RELNOTE)"
endif


docker:
	@echo "Base Images is set to: $(BASEIMAGE)"
	@echo "Building: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	$(BUILDER) build -t "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)" --target $(BUILDSTAGE) --build-arg BASEIMAGE=$(BASEIMAGE) .

push:   
	@echo "Pushing: $(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"
	$(BUILDER) push "$(REGISTRY)/$(IMAGENAME):$(IMAGETAG)"

build-base-image: download-csm-common
	$(eval include csm-common.mk)
	@echo "Building base image from $(DEFAULT_BASEIMAGE) and loading dependencies..."
	./scripts/build_ubi_micro.sh $(DEFAULT_BASEIMAGE)
	@echo "Base image build: SUCCESS"
	$(eval BASEIMAGE=localhost/vgs-ubimicro:latest)

download-csm-common:
	curl -O -L https://raw.githubusercontent.com/dell/csm/main/config/csm-common.mk
