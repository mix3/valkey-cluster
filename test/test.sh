#!/bin/bash

set -e

echo "VALKEY_VERSION=${VALKEY_VERSION:-latest}"
echo "VALKEY_VARIANT=$VALKEY_VARIANT"

services=$(yq '... comments="" | .services | keys' compose.yml | grep -v test-runner- | sed 's/- //g')
testrunner1=$(yq '... comments="" | .services | keys' compose.yml | grep test-runner-1 | sed 's/- //g')
testrunner2=$(yq '... comments="" | .services | keys' compose.yml | grep test-runner-2 | sed 's/- //g')

# init
docker compose down -v
docker compose build

# launch
for h in $(echo "$services")
do
	docker compose up -d $h
done
docker compose up -d $(echo $services | xargs) --wait

# test-1
for h in $(echo "$testrunner1")
do
	docker compose run --rm $h
done

# relaunch
docker compose down
for h in $(echo "$services" | awk 'NR%2==0' && echo "$services" | awk 'NR%2!=0')
do
	docker compose up -d $h
done
docker compose up -d $(echo $services | xargs) --wait

# test-2
for h in $(echo "$testrunner2")
do
	docker compose run --rm $h
done

# check moduleload
if [ "$VALKEY_VARIANT" == "-bundle" ];  then
	for h in $(docker-compose exec default bash -c "ls /usr/lib/valkey/*.so")
	do
		docker-compose exec default valkey-cli -c -p 7000 module list | grep -A 1 path | grep $h
	done
fi
