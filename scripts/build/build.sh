#!/bin/bash

export GPG_FINGERPRINT=298A405CE1C450D2

echo "Prefetching key"

while ! echo "test" | gpg --armor --detach-sign; do
	echo "Testing again"
	sleep 1
done

rm -r dist
goreleaser release --clean
