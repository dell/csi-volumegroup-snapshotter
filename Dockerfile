ARG BASEIMAGE
ARG GOIMAGE


FROM ${GOIMAGE} as builder
RUN mkdir -p /go/src
COPY ./ /go/src/
WORKDIR /go/src/
RUN CGO_ENABLED=0 \
    make build

# Final Stage
FROM $BASEIMAGE AS final

# Copy the binary from the builder stage

COPY --from=builder /go/src/bin/vg-snapshotter .

# Set entry point
ENTRYPOINT ["/vg-snapshotter"]

# Metadata labels
LABEL vendor="Dell Technologies" \
      maintainer="Dell Technologies" \
      name="dellcsi-vg-snapshotter" \
      summary="CSI VG Snapshotter for Dell EMC PowerFlex/PowerStore" \
      description="Dell Storage VolumeGroup Snapshot Controller for CSI" \
      release="1.13.0" \
      version="1.8.0" \
      license="Apache-2.0"
COPY licenses licenses/
