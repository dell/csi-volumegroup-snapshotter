#sha256:8c5723a40221c3e620a86e7ecba02b7af524e6c1960c83f46d361fa03a5eaaee
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
