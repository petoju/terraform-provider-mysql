#!/bin/bash

if [ -e ".env" ]; then
	source ./.env
fi

if [ -z "$GITHUB_TOKEN" ]; then
	echo "No github token!"
	exit 1
fi

# Debug with gpg --card-status
# Initialize signing.
echo "Test" | gpg --armor --detach-sign

export GPG_AGENT_SOCKET=$(gpgconf --list-dirs agent-socket)
echo "Using GPG Agent Socket: ${GPG_AGENT_SOCKET}"

DOCKER_IMAGE="$(docker build -q .)"

docker run -e GITHUB_TOKEN -v "${GPG_AGENT_SOCKET}:/home/user/.gnupg/S.gpg-agent:rw" -v "$PWD/../../:/home/user/app" -it "$DOCKER_IMAGE"

git push
git tag -s "$TAG" -m "Update to $TAG"
git push origin "$TAG"
