# Dev image used by Tilt's live-update loop.
#
# Tilt builds the provider binary on the host (see the `provider-build`
# local_resource in dev/Tiltfile), copies only bin/provider into this image,
# and then live-syncs the rebuilt binary into the running container on each
# change without recreating the pod.
#
# A slim Debian base (rather than distroless) is used so the restart_process
# wrapper can re-exec the binary on live update.
FROM debian:bookworm-slim AS dev

WORKDIR /home/provider

# Populated by Tilt from the host-built binary (only=['bin/provider']).
COPY bin/provider /home/provider/provider

USER 65532:65532

ENTRYPOINT ["/home/provider/provider"]
