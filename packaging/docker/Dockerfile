FROM golang:1.22-bookworm AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOFLAGS='-trimpath -buildvcs=false' go build -ldflags='-buildid=' -o /out/patchline ./cmd/patchline

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/patchline /usr/local/bin/patchline
ENTRYPOINT ["/usr/local/bin/patchline"]
CMD ["--help"]
