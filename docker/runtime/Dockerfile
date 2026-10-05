ARG DEV_BASE_IMAGE=ghcr.io/thinkerqaq/dev-base@sha256:f922f24b2dd917faa87bfc090dd90321aaf82651cc9a2de320a22519c3b77a8e

FROM ${DEV_BASE_IMAGE} AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/devtool ./cmd/devtool

FROM ${DEV_BASE_IMAGE}
COPY --from=build /out/devtool /usr/local/bin/devtool
