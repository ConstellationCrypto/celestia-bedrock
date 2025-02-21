#! /bin/bash
set -e #u

if [ "$#" -ne 1 ]; then echo "Usage: ./upload_all_images VERSION_TAG"; exit; fi

VERSION=$1
ACCOUNT=001138754299 #$(aws sts get-caller-identity --query Account --output text)
aws ecr get-login-password --region us-west-2 --profile Constellation-Admin/PowerUser  | docker login --username AWS --password-stdin 001138754299.dkr.ecr.us-west-2.amazonaws.com
build_tag_push () {
  echo $1
  docker build -t $1 -f $1/Dockerfile $2 &&
  docker tag $1 $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/$1:$VERSION &&
  docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/$1:$VERSION
}

build_tag_push_op_pipe () {
  make golang-docker && echo OK || echo "Failed build_tag_push_op_pipe"
  GIT_COMMIT=$(git rev-parse HEAD)
  docker tag us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:$GIT_COMMIT $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-node:$VERSION &&
  docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-node:$VERSION
  docker tag us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:$GIT_COMMIT docker.io/library/op-node:latest #needed for bedrock-deployer

  # docker tag us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:$GIT_COMMIT $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-batcher:$VERSION &&
  # docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-batcher:$VERSION

  docker tag us-docker.pkg.dev/oplabs-tools-artifacts/images/op-proposer:$GIT_COMMIT $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-proposer:$VERSION &&
  docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-proposer:$VERSION

  docker tag us-docker.pkg.dev/oplabs-tools-artifacts/images/op-dispute-mon:$GIT_COMMIT $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-dispute-mon:$VERSION &&
  docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-dispute-mon:$VERSION

  docker tag us-docker.pkg.dev/oplabs-tools-artifacts/images/op-challenger:$GIT_COMMIT $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-challenger:$VERSION &&
  docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-challenger:$VERSION

}

docker build -t us-docker.pkg.dev/oplabs-tools-artifacts/images/op-stack-go:latest -f ops/docker/op-stack-go/Dockerfile .
build_tag_push_op_pipe
build_tag_push op-geth op-geth
# bedrock-deployer depends on the op-node, op-geth images
#build_tag_push bedrock-deployer .
