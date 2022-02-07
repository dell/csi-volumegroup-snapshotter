#sha256:3aa3f379a81013bd3264faa0af87d201cdaa5981050d78c567b48fdfd5b38bb8
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.5-230

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
