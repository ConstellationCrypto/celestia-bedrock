# This Dockerfile builds all the dependencies needed by the monorepo, and should
# be used to build any of the follow-on services
#

# Stage 0 (named `manifests`) collects
# dependency manifest files (`package.json` and `pnpm-lock.yaml`) which are then
# used by stage 1 to install these dependencies
# development. The only reason we need a separate stage just for collecting the
# dependency manifests is that Docker's `COPY` command still does not allow
# copying based on a glob pattern (see this GitHub issue for more details
# https://github.com/moby/moby/issues/15858). Being able to copy only manifests
# into stage 1 (the `COPY --from=manifests` statement) is important to maximize
# Docker build cache hit rate. `alpine` is chosen as the base image for the
# first stage because it's the smallest image that have access to the `cp
# --parents -t` command (by installing the `coreutils` package).
FROM alpine:3.16 as manifests
RUN apk add coreutils

WORKDIR /tmp
COPY pnpm-lock.yaml pnpm-workspace.yaml .nvmrc package.json ./src/
COPY packages src/packages/
RUN mkdir manifests && \
  cd src && \
  # copy package.json recursively
  find . -name 'package.json' | xargs cp --parents -t ../manifests/ && \
  # pnpm-lock.yaml
  cp pnpm-lock.yaml ../manifests/ && \
  # pnpm-workspace.yaml
  cp pnpm-workspace.yaml ../manifests/ && \
  # .nvmrc
  cp .nvmrc ../manifests/

FROM op-geth as op-geth
FROM op-node as op-node
FROM ghcr.io/foundry-rs/foundry:nightly-ee562e889940341f11707f9c3e933a87719ca25d as foundry
# bullseye-slim is debian based
# we use it rather than alpien because it's not much
# bigger and alpine is often missing packages for node applications
# alpine is not officially supported by node.js
FROM node:23.9.0-bullseye-slim as base

RUN apt-get update && apt-get install -y \
  curl \
  jq \
  python3 \
  ca-certificates \
  git \
  g++ \
  make \
  gcc \
  musl-dev \
  bash \
  # the following 4 deps are needed for node-hid
  # which is a deep sub dependency of ethers to install
  # correctly
  pkg-config \
  libusb-1.0-0-dev \
  libudev-dev \
  --no-install-recommends

RUN npm install pnpm --global

COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast

WORKDIR /opt/optimism

# Copy manifest files into the image in
# preparation for `pnpm install`.
COPY --from=manifests /tmp/manifests  ./
COPY *.json ./
COPY .pnpmfile.cjs .

RUN pnpm install --filter contracts-bedrock
COPY ./packages/contracts-bedrock ./packages/contracts-bedrock
WORKDIR /opt/optimism/packages/contracts-bedrock
RUN pnpm clean
RUN pnpm build
COPY ./packages/deploy.ts deploy.ts

COPY --from=op-geth /usr/local/bin/geth /usr/local/bin/geth
COPY --from=op-node /usr/local/bin/op-node /usr/local/bin/op-node

CMD ["npx", "ts-node", "deploy.ts"]
