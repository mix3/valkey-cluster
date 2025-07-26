initial_port="$1"
max_port="$2"
require_cluster_create="$3"
slaves_per_master="$4"
nodes="$5"
sentinel="$6"
masters="$7"

program_valkey_template ()
{
  local count=$1
  local port=$2
  echo "

[program:valkey-$count]
command=/prefix-output.sh /usr/local/bin/valkey-server /valkey-conf/$port/valkey.conf
stdout_logfile=/dev/stdout
stderr_logfile=/dev/stderr
stdout_maxbytes=0
stderr_maxbytes=0
stdout_logfile_maxbytes = 0
stderr_logfile_maxbytes = 0
autorestart=true
"
}

program_sentinel_template ()
{
  local count=$1
  local port=$2
  echo "

[program:valkey-sentinel-$count]
command=/prefix-output.sh /usr/local/bin/valkey-sentinel /valkey-conf/sentinel-$port.conf
stdout_logfile=/dev/stdout
stderr_logfile=/dev/stderr
stdout_maxbytes=0
stderr_maxbytes=0
stdout_logfile_maxbytes = 0
stderr_logfile_maxbytes = 0
autorestart=true
"
}

result_str="
[unix_http_server]
file=/tmp/supervisor.sock                       ; path to your socket file

[supervisord]
logfile=/supervisord.log                        ; supervisord log file
logfile_maxbytes=50MB                           ; maximum size of logfile before rotation
logfile_backups=10                              ; number of backed up logfiles
loglevel=error                                  ; info, debug, warn, trace
pidfile=/var/run/supervisord.pid                ; pidfile location
nodaemon=true                                   ; run supervisord as a nodaemon
minfds=1024                                     ; number of startup file descriptors
minprocs=200                                    ; number of process descriptors
user=root                                       ; default user
childlogdir=/                                   ; where child log files will live

[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

[supervisorctl]
serverurl=unix:///tmp/supervisor.sock         ; use a unix:// URL  for a unix socket
"

count=1
for port in `seq $initial_port $max_port`; do
  result_str="$result_str$(program_valkey_template $count $port)"
  count=$((count + 1))
done

if [ "$require_cluster_create" = "true" ]; then
  result_str="$result_str

[program:valkey-cluster-create]
command=/prefix-output.sh /valkey-cluster-create.sh '$slaves_per_master' '$nodes'
stdout_logfile=/dev/stdout
stderr_logfile=/dev/stderr
stdout_maxbytes=0
stderr_maxbytes=0
stdout_logfile_maxbytes = 0
stderr_logfile_maxbytes = 0
autostart=true
autorestart=false
"
fi

if [ "$sentinel" = "true" ]; then
  count=1
  for port in $(seq $initial_port $(($initial_port + $masters - 1))); do
    result_str="$result_str$(program_sentinel_template $count $port)"
    count=$((count + 1))
  done
fi

echo "$result_str"
