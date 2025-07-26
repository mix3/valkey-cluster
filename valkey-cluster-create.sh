#!/bin/sh
SLAVES_PER_MASTER="$1"
nodes="$2"

# wait to setup node
for h in $(echo "$nodes")
do
        while ! valkey-cli -h $(echo $h | cut -d':' -f1) -p $(echo $h | cut -d':' -f2) ping > /dev/null 2>&1
        do
                sleep 1
        done
done

echo "Using valkey-cli to create the cluster"
echo "yes" | eval valkey-cli --cluster create --cluster-replicas "$SLAVES_PER_MASTER" "$nodes"
