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

build-base-image:
	@echo "Building base image from $(BASEIMAGE) and loading dependencies..."
	./scripts/build_ubi_micro.sh $(BASEIMAGE)
	@echo "Base image build: SUCCESS"
	$(eval BASEIMAGE=localhost/vgs-ubimicro:latest)
