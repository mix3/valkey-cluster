#!/bin/sh

if [ "$1" = 'valkey-cluster' ]; then
    # Allow passing in cluster IP by argument or environmental variable
    IP="${2:-$IP}"

    if [ -z "$IP" ]; then # If IP is unset then discover it
        IP=$(hostname -I)
    fi

    echo " -- IP Before trim: '$IP'"
    IP=$(echo ${IP}) # trim whitespaces
    echo " -- IP Before split: '$IP'"
    IP=${IP%% *} # use the first ip
    echo " -- IP After trim: '$IP'"

    if [ -z "$INITIAL_PORT" ]; then # Default to port 7000
      INITIAL_PORT=7000
    fi

    if [ -z "$MASTERS" ]; then # Default to 3 masters
      MASTERS=3
    fi

    if [ -z "$SLAVES_PER_MASTER" ]; then # Default to 1 slave for each master
      SLAVES_PER_MASTER=1
    fi

    if [ -z "$BIND_ADDRESS" ]; then # Default to any IPv4 address
      BIND_ADDRESS=0.0.0.0
    fi

    max_port=$(($INITIAL_PORT + $MASTERS * ( $SLAVES_PER_MASTER  + 1 ) - 1))
    first_standalone=$(($max_port + 1))
    if [ "$STANDALONE" = "true" ]; then
      STANDALONE=2
    fi
    if [ ! -z "$STANDALONE" ]; then
      max_port=$(($max_port + $STANDALONE))
    fi

    if ! find /valkey-data/* -type d >/dev/null 2>&1; then
      REQUIRE_CLUSTER_CREATE="true"
    fi

    for port in $(seq $INITIAL_PORT $max_port); do
      mkdir -p /valkey-conf/${port}
      mkdir -p /valkey-data/${port}

      if [ "$port" -lt "$first_standalone" ]; then
        PORT=${port} BIND_ADDRESS=${BIND_ADDRESS} envsubst < /valkey-conf/valkey-cluster.tmpl > /valkey-conf/${port}/valkey.conf
        nodes="$nodes $IP:$port"
      else
        PORT=${port} BIND_ADDRESS=${BIND_ADDRESS} envsubst < /valkey-conf/valkey.tmpl > /valkey-conf/${port}/valkey.conf
      fi

      if [ "$port" -lt $(($INITIAL_PORT + $MASTERS)) ]; then
        if [ "$SENTINEL" = "true" ]; then
          PORT=${port} SENTINEL_PORT=$((port - 2000)) envsubst < /valkey-conf/sentinel.tmpl > /valkey-conf/sentinel-${port}.conf
          cat /valkey-conf/sentinel-${port}.conf
        fi
      fi

    done

    bash /generate-supervisor-conf.sh "$INITIAL_PORT" "$max_port" "$REQUIRE_CLUSTER_CREATE" "$SLAVES_PER_MASTER" "$nodes" "$SENTINEL" "$MASTERS" > /etc/supervisor/supervisord.conf

    exec supervisord -c /etc/supervisor/supervisord.conf
else
  exec "$@"
fi
