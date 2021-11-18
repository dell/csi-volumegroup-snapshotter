#sha256:e0814339ffc6c933652bed0c5f8b6416b9a3d40be2f49f95e6e4128387d2a24a
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.5-204

LABEL vendor="Dell Inc." \
      name="dellcsi-vg-snapshotter" \
      summary="CSI VG Snapshotter for Dell EMC Powerflex" \
      description="Dell Storage VolumeGroup Snapshot Controller for CSI" \
      version="0.3.0" \
      license="Apache-2.0"

#COPY licenses /licenses

RUN microdnf update -y && microdnf install -y tar gzip

COPY ./bin/vg-snapshotter .
ENTRYPOINT ["/vg-snapshotter"]
