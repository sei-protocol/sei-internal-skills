# syntax=docker/dockerfile:1
#
# sei-omnigent — the Sei Omnigent overlay server image (PLT-735).
#
# Two-stage. The builder resolves the overlay + its pinned deps (omnigent==0.6.0,
# sei_omnigent, uvicorn) into a relocatable prefix; the runtime is a clean
# python:3.12-slim. The k8s probes poll /health with python-urllib (the image
# guarantees python), so no curl is added — the consuming walle manifest's exec
# probes use python, matching the host-arm-job convention.
#
# slim, NOT distroless: the operator chose slim for a real shell for debug exec.
# The trade is a larger surface; the deploy offsets it with restricted
# PodSecurity + readOnlyRootFilesystem.

# ---- builder ---------------------------------------------------------------
FROM python:3.12-slim AS builder

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /src

# Build context is sei_omnigent/. Copy the builder inputs explicitly — the
# .dockerignore denies-all then re-includes pyproject/src + the agent bundles;
# naming these avoids buildkit's CopyIgnoredFile warning on a bare `COPY . .`.
COPY pyproject.toml ./
COPY src ./src

# Resolve into a relocatable prefix. `.` builds the overlay from pyproject, which
# pulls omnigent==0.6.0 + uvicorn (declared deps). --prefix=/install keeps the
# tree copyable into the runtime without dragging build tooling along.
#
# No --pre: the pre-release allowance is SCOPED to exactly the OTel
# instrumentation packages via explicit `==0.63b1` pins in pyproject.toml (pip
# honors an exact-version pin to a pre-release without the global flag); the rest
# of the closure resolves to stable finals only. See the dependencies block in
# pyproject.toml for the rationale + the on-bump re-evaluation note.
RUN pip install --prefix=/install .

# ---- runtime ---------------------------------------------------------------
FROM python:3.12-slim AS runtime

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Non-root uid:gid 1000:1000 WITH a real /etc/passwd entry + HOME. The harness
# sets OMNIGENT_DATA_DIR=/data so the DB path no longer needs $HOME, but
# Path.home() / pwd.getpwuid(1000) must still resolve (no KeyError). The uid
# matches the consuming walle manifest's runAsUser/fsGroup: 1000.
RUN useradd --uid 1000 --create-home --home-dir /home/omnigent omnigent
ENV HOME=/home/omnigent

# The resolved env (overlay + omnigent + uvicorn) from the builder.
COPY --from=builder /install /usr/local

# The xreview agent bundle (config.yaml + skill body), baked at a stable path so
# the deploy can register it via OMNIGENT_BUILTIN_AGENT_DIRS (the server tars +
# registers the dir at startup; the whole bundle incl. skills/ must be present).
# Owned by uid 1000 (the runtime user); read-only at runtime.
COPY --chown=1000:1000 agents/xreview /opt/sei-omnigent/agents/xreview

# The root-cause bundle, baked at the same path convention for
# OMNIGENT_BUILTIN_AGENT_DIRS registration. Its Grafana-MCP signal source is a
# runtime sidecar, not baked here.
COPY --chown=1000:1000 agents/root-cause /opt/sei-omnigent/agents/root-cause

USER 1000:1000

# No HEALTHCHECK — k8s liveness/readiness/startup probes own health (the in-image
# HEALTHCHECK would double-probe and muddy pod status). The app writes only under
# /data (PVC) and /tmp (emptyDir); the runtime root is readOnly in the manifest.
#
# Equivalent to the `sei-omnigent-serve` console script; -m is explicit so the
# command is legible in the manifest and independent of PATH shims.
ENTRYPOINT ["python", "-m", "sei_omnigent.server.serve_main"]
