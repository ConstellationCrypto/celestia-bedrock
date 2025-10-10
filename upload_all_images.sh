#! /bin/bash
set -e #u

if [ "$#" -ne 1 ]; then echo "Usage: ./upload_all_images VERSION_TAG"; exit; fi

VERSION=$1
ACCOUNT=001138754299 #$(aws sts get-caller-identity --query Account --output text)
aws ecr get-login-password --region us-west-2 --profile Constellation-Admin/PowerUser  | docker login --username AWS --password-stdin 001138754299.dkr.ecr.us-west-2.amazonaws.com

build_tag_push_op_pipe () {
  make golang-docker && echo OK || echo "Failed build_tag_push_op_pipe"
  GIT_COMMIT=$(git rev-parse HEAD)

  docker tag us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:$GIT_COMMIT $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-batcher:$VERSION 
  docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/op-batcher:$VERSION

}

build_tag_push_op_pipe
