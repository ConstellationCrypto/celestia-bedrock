FROM alpine:3.16 as manifests
RUN apk add coreutils

WORKDIR /tmp
COPY yarn.lock .nvmrc package.json ./src/
COPY packages src/packages/
RUN mkdir manifests && \
  cd src && \
  # copy package.json recursively
  find . -name 'package.json' | xargs cp --parents -t ../manifests/ && \
  # yarn.lock
  cp yarn.lock ../manifests/ && \
  # .nvmrc
  cp .nvmrc ../manifests/

FROM ethereumoptimism/foundry:latest as foundry

FROM op-geth as op-geth
FROM op-node as op-node

FROM node:16-alpine3.14 as base

COPY --from=op-geth /usr/local/bin/geth /usr/local/bin/geth
COPY --from=op-node /usr/local/bin/op-node /usr/local/bin/op-node

# Base: install deps
RUN apk --no-cache add curl \
  jq \
  python3 \
  ca-certificates \
  git \
  make \
  gcc \
  musl-dev \
  linux-headers \
  bash \
  build-base \
  gcompat

ENV GLIBC_KEY=https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub
ENV GLIBC_KEY_FILE=/etc/apk/keys/sgerrand.rsa.pub
ENV GLIBC_RELEASE=https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.35-r0/glibc-2.35-r0.apk

RUN wget -q -O ${GLIBC_KEY_FILE} ${GLIBC_KEY} \
  && wget -O glibc.apk ${GLIBC_RELEASE} \
  && apk add glibc.apk --force

COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast

WORKDIR /opt/optimism
#COPY packages/core-utils/package.json ./packages/core-utils/package.json
#COPY packages/contracts-bedrock/package.json ./packages/contracts-bedrock/package.json
#COPY packages/hardhat-deploy-config/package.json ./packages/hardhat-deploy-config/package.json
COPY --from=manifests /tmp/manifests  ./
COPY *.json ./

RUN yarn install --frozen-lockfile && yarn cache clean

#COPY ./packages/core-utils ./packages/core-utils
#COPY ./packages/contracts-bedrock ./packages/contracts-bedrock
#COPY ./packages/hardhat-deploy-config ./packages/hardhat-deploy-config
COPY ./packages ./packages
RUN yarn build

WORKDIR /opt/optimism/packages/contracts-bedrock
CMD ["npx", "ts-node", "deploy.ts"]
