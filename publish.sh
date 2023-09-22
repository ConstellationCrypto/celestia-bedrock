#! /bin/bash
set -e #u

if [ "$#" -ne 1 ]; then echo "Usage: ./upload_all_images VERSION_TAG"; exit; fi

VERSION=$1
ACCOUNT=001138754299
REGION=us-west-2
TAG=celestia-$VERSION

make build-ts
pnpm lerna publish $VERSION --preid=celestia --no-private --force-publish @constellation-labs/bedrock-sdk --force-publish @constellation-labs/contracts-bedrock --ignore-changes '**'

aws ecr get-login-password --region $REGION | docker login --username AWS --password-stdin $ACCOUNT.dkr.ecr.$REGION.amazonaws.com

build_tag_push () {
  echo $1
  docker build -t $1 -f $1/Dockerfile $2 &&
  docker tag $1 $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/$1:$TAG &&
  docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/$1:$TAG
}

build_tag_push op-geth op-geth
# bedrock-deployer depends on the op-geth image
build_tag_push bedrock-deployer .
# op-node, op-proposer, and op-batcher depend on the bedrock-deployer image
build_tag_push op-node .
build_tag_push op-proposer .
build_tag_push op-batcher .
