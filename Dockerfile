#sha256:c8c1c0f893a7ba679fd65863f2d1389179a92957c31e95521b3290c6b6fc4a76
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6-902.1661794353

LABEL vendor="Dell Inc." \
      name="dellcsi-vg-snapshotter" \
      summary="CSI VG Snapshotter for Dell EMC Powerflex" \
      description="Dell Storage VolumeGroup Snapshot Controller for CSI" \
      version="1.0.0" \
      license="Apache-2.0"

#COPY licenses /licenses

RUN microdnf update -y && microdnf install -y tar gzip
COPY ./bin/vg-snapshotter .
ENTRYPOINT ["/vg-snapshotter"]
