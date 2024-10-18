# This Dockerfile builds all the dependencies needed by the monorepo, and should
# be used to build any of the follow-on services
#

FROM alpine:3.16 as manifests
RUN apk add coreutils

WORKDIR /tmp

FROM op-geth as op-geth
FROM op-node as op-node
FROM ghcr.io/foundry-rs/foundry:nightly-143abd6a768eeb52a5785240b763d72a56987b4a as foundry
# bullseye-slim is debian based
# we use it rather than alpien because it's not much
# bigger and alpine is often missing packages for node applications
# alpine is not officially supported by node.js
FROM node:22.0.0-bullseye-slim as base

RUN apt-get update && apt-get install -y \
  wget \
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

COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast

WORKDIR /opt/optimism

COPY versions.json /opt/optimism/
RUN \
  wget -O - https://github.com/casey/just/releases/download/$(jq -r .just < ./versions.json)/just-$(jq -r .just < ./versions.json)-x86_64-unknown-linux-musl.tar.gz \
  | tar --directory /usr/local/bin --extract --gzip --file - just

COPY ./packages/contracts-bedrock ./packages/contracts-bedrock
WORKDIR /opt/optimism/packages/contracts-bedrock
RUN just clean
RUN just build
COPY ./packages/tsconfig.json tsconfig.json
COPY ./packages/package.json package.json
COPY ./packages/deploy.ts deploy.ts
RUN npm install pnpm --global
RUN pnpm install
COPY --from=op-geth /usr/local/bin/geth /usr/local/bin/geth
COPY --from=op-node /usr/local/bin/op-node /usr/local/bin/op-node

CMD ["npx", "ts-node", "deploy.ts"]
