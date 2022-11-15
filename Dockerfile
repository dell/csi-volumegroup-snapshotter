#sha256:c5ffdf5938d73283cec018f2adf59f0ed9f8c376d93e415a27b16c3c6aad6f45
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6-994

LABEL vendor="Dell Inc." \
      name="dellcsi-vg-snapshotter" \
      summary="CSI VG Snapshotter for Dell EMC Powerflex" \
      description="Dell Storage VolumeGroup Snapshot Controller for CSI" \
      version="1.2.0" \
      license="Apache-2.0"

#COPY licenses /licenses

RUN microdnf update -y && microdnf install -y tar gzip
COPY ./bin/vg-snapshotter .
ENTRYPOINT ["/vg-snapshotter"]
