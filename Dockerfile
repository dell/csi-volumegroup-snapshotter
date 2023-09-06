ARG BASEIMAGE

FROM $BASEIMAGE AS final 
COPY ./bin/vg-snapshotter .
ENTRYPOINT ["/vg-snapshotter"]

LABEL vendor="Dell Inc." \
      name="dellcsi-vg-snapshotter" \
      summary="CSI VG Snapshotter for Dell EMC Powerflex" \
      description="Dell Storage VolumeGroup Snapshot Controller for CSI" \
      version="1.2.0" \
      license="Apache-2.0"

