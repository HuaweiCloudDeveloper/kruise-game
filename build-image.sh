set -euo pipefail
TAG='v0.10.0-0903'
#IMAGE="swr.cn-southwest-2.myhuaweicloud.com/mabing/kruise-game-manager:${TAG}"
IMAGE="swr.ap-southeast-1.myhuaweicloud.com/huaweiclouddeveloper/kruise-game-manager:v0.10.0-hw-${TAG}"
docker build . -f Dockerfile.debug -t "${IMAGE}"
docker save "${IMAGE}" -o /tmp/${TAG}.tar.gz