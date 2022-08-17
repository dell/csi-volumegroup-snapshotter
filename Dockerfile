#sha256:d1f8eff6032334a81d7cbfd73dacee680e8138db57ecbc91548b97bb45e698e5
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6-902

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
