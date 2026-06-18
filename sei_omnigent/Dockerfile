# syntax=docker/dockerfile:1
#
# sei-tide-omnigent — the Sei Omnigent overlay server image (PLT-735).
#
# Two-stage. The builder resolves the overlay + its pinned deps (omnigent==0.1.1,
# sei_omnigent, uvicorn) into a relocatable prefix; the runtime is a clean
# python:3.12-slim with ONLY curl added (the k8s probe is `curl -fsS
# http://127.0.0.1:8443/health` — curl MUST be on PATH or the probe errors).
#
# slim, NOT distroless: the operator chose slim for shell/tooling availability
# (curl, a real shell for debug exec). The trade is a larger surface; the deploy
# offsets it with restricted PodSecurity + readOnlyRootFilesystem.

# ---- builder ---------------------------------------------------------------
FROM python:3.12-slim AS builder

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /src

# Build context is sei_omnigent/ — pyproject + the src/ package are self-contained.
# Copy them explicitly (the .dockerignore already prunes the context to these;
# naming them avoids buildkit's CopyIgnoredFile warning on a bare `COPY . .`).
COPY pyproject.toml ./
COPY src ./src

# Resolve into a relocatable prefix. `.` builds the overlay from pyproject, which
# pulls omnigent==0.1.1 + uvicorn (declared deps). --prefix=/install keeps the
# tree copyable into the runtime without dragging build tooling along.
#
# --pre is REQUIRED, not optional: omnigent==0.1.1 constrains its
# opentelemetry-instrumentation-* deps as `<1,>=0`, but those projects publish
# ONLY pre-release (0.XXbN) versions on PyPI. Without --pre, pip excludes them
# and resolution fails with "No matching distribution". This is an UPSTREAM
# packaging quirk of the pinned omnigent release (flagged to the operator); the
# Tide CI verify jobs never hit it because they install --no-deps. Re-evaluate
# dropping --pre if a future omnigent pin tightens those constraints to finals.
RUN pip install --pre --prefix=/install .

# ---- runtime ---------------------------------------------------------------
FROM python:3.12-slim AS runtime

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# curl for the k8s probe (`curl -fsS http://127.0.0.1:8443/health`). --no-install-recommends
# + cleaning the apt lists in the same layer keeps the add minimal.
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

# Non-root uid:gid 1000:1000 WITH a real /etc/passwd entry + HOME. The harness
# sets OMNIGENT_DATA_DIR=/data so the DB path no longer needs $HOME, but
# Path.home() / pwd.getpwuid(1000) must still resolve (no KeyError). The uid MUST
# match the deploy manifest's runAsUser: 1000.
RUN useradd --uid 1000 --create-home --home-dir /home/omnigent omnigent
ENV HOME=/home/omnigent

# The resolved env (overlay + omnigent + uvicorn) from the builder.
COPY --from=builder /install /usr/local

USER 1000:1000

# No HEALTHCHECK — k8s liveness/readiness/startup probes own health (the in-image
# HEALTHCHECK would double-probe and muddy pod status). The app writes only under
# /data (PVC) and /tmp (emptyDir); the runtime root is readOnly in the manifest.
#
# Equivalent to the `sei-omnigent-serve` console script; -m is explicit so the
# command is legible in the manifest and independent of PATH shims.
ENTRYPOINT ["python", "-m", "sei_omnigent.server.serve_main"]
