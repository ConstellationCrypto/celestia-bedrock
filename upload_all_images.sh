#! /bin/bash
set -e #u

if [ "$#" -ne 1 ]; then echo "Usage: ./upload_all_images VERSION_TAG"; exit; fi

VERSION=$1
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
REGION=us-west-2

aws ecr get-login-password --region $REGION | docker login --username AWS --password-stdin $ACCOUNT.dkr.ecr.$REGION.amazonaws.com

build_tag_push () { docker build -t $1 -f $1/Dockerfile $2 && docker tag $1 $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/$1:$VERSION && docker image push $ACCOUNT.dkr.ecr.us-west-2.amazonaws.com/$1:$VERSION; }

build_tag_push op-batcher .
build_tag_push op-node .
build_tag_push op-proposer .
build_tag_push op-geth op-geth
build_tag_push bedrock-deployer .
