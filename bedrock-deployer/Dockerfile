# bullseye-slim is debian based
# we use it rather than alpine because it's not much
# bigger and alpine is often missing packages for node applications
# alpine is not officially supported by node.js
FROM node:16.16.0-bullseye-slim

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

# install go
COPY --from=golang:1.20.13-alpine3.19 /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

# download op-geth modules
COPY ./op-geth/go.mod /app/op-geth/go.mod
COPY ./op-geth/go.sum /app/op-geth/go.sum
WORKDIR /app/op-geth
RUN go mod download

# build abigen
COPY ./op-geth /app/op-geth
RUN go build -buildvcs=false -o /usr/local/bin/abigen ./cmd/abigen

# download go modules
COPY ./go.mod /app/go.mod
COPY ./go.sum /app/go.sum
WORKDIR /app
RUN go mod download

# install npm packages
RUN npm install pnpm --global
COPY pnpm-lock.yaml pnpm-workspace.yaml .nvmrc package.json .
COPY packages/contracts-bedrock/package.json packages/contracts-bedrock/package.json
RUN pnpm install --frozen-lockfile --filter contracts-bedrock

# install foundry
COPY --from=ghcr.io/foundry-rs/foundry:v1.0.0 /usr/local/bin/forge /usr/local/bin/cast /usr/local/bin

# build contracts
COPY packages/contracts-bedrock packages/contracts-bedrock
WORKDIR /app/packages/contracts-bedrock
RUN pnpm clean && pnpm build

# generate l2 predeploy contracts
COPY op-bindings /app/op-bindings
WORKDIR /app/op-bindings
RUN make clean bindings-build

# build op-node genesis subcommand
COPY op-node /app/op-node
COPY op-service /app/op-service
COPY op-chain-ops /app/op-chain-ops
WORKDIR /app/op-chain-ops
RUN go build -o /usr/local/bin/op-node ./cmd/genesis

# copy op-geth
COPY --from=op-geth /usr/local/bin/geth /usr/local/bin/geth

# copy tsconfig.json
COPY tsconfig.json /app/tsconfig.json

# remove go files (except for op-bindings which is exported to other containers)
RUN rm -rf /root/go /root/.cache/go-build /usr/local/go /app/op-geth /app/op-node /app/op-chain-ops /app/op-service

WORKDIR /app/packages/contracts-bedrock
CMD ["npx", "ts-node", "deploy.ts"]
