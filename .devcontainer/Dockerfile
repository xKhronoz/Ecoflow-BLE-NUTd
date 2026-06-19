FROM mcr.microsoft.com/devcontainers/go:1.26-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        netcat-openbsd \
        nut-client \
    && rm -rf /var/lib/apt/lists/*
