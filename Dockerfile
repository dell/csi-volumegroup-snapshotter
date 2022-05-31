#sha256:21504085e8d740e62b52573fe9a1a0d58a3e7dba589cac69734ad2fa81d66635
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6-751

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
