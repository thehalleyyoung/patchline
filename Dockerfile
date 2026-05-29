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

WORKDIR /workspace/patchline
COPY go.mod ./
RUN go mod download
COPY . .

CMD ["make", "artifact-smoke"]
