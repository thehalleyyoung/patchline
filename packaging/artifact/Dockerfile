FROM golang:1.22-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        jq \
        make \
        z3 \
    && rm -rf /var/lib/apt/lists/*

ENV PATCHLINE_ARTIFACT_MODE=container
ENV PATCHLINE_RESULTS_DIR=/workspace/patchline/results/generated/artifact-container-rebuild
ENV PATCHLINE_DOWNLOAD_DIR=/workspace/patchline/results/generated/artifact-container-cache

WORKDIR /workspace/patchline
COPY go.mod ./
RUN go mod download
COPY . .

CMD ["bash", "scripts/artifact-container-rebuild.sh"]
