#!/bin/bash

set -e

echo "VALKEY_VERSION=${VALKEY_VERSION:-latest}"
echo "VALKEY_VARIANT=$VALKEY_VARIANT"

docker compose build

services=$(yq '.services | keys' compose.yml | sed 's/- //g')

for h in $(echo "$services")
do
	docker compose up -d $h
done

docker compose up -d --wait

docker compose down

for h in $(echo "$services" | awk 'NR%2==0' && echo "$services" | awk 'NR%2!=0')
do
	docker compose up -d $h
done

docker compose up -d --wait

if [ "$VALKEY_VARIANT" == "-bundle" ];  then
	for h in $(docker-compose exec default bash -c "ls /usr/lib/valkey/*.so")
	do
		docker-compose exec default valkey-cli -c -p 7000 module list | grep -A 1 path | grep $h
	done
fi
