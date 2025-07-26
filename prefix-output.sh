#!/bin/bash

# Get prefix from SUPERVISOR_PROCESS_NAME environment variable
printf -v PREFIX "%-21.21s" "${SUPERVISOR_PROCESS_NAME}"

# Array of color codes (Docker-compose style)
declare -a COLORS=(
    '\033[31m'  # Red
    '\033[32m'  # Green
    '\033[33m'  # Yellow
    '\033[34m'  # Blue
    '\033[35m'  # Magenta
    '\033[36m'  # Cyan
    '\033[37m'  # White
    '\033[91m'  # Bright Red
    '\033[92m'  # Bright Green
    '\033[93m'  # Bright Yellow
    '\033[94m'  # Bright Blue
    '\033[95m'  # Bright Magenta
    '\033[96m'  # Bright Cyan
)

# Reset code
RESET='\033[0m'

# Calculate a hash value from the prefix to determine the color
hash_prefix() {
    local sum=0
    local i
    for (( i=0; i<${#PREFIX}; i++ )); do
        sum=$((sum + $(printf "%d" "'${PREFIX:$i:1}")))
    done
    echo $((sum % ${#COLORS[@]}))
}

# Get the color index
COLOR_INDEX=$(hash_prefix)
COLOR=${COLORS[$COLOR_INDEX]}

# Prefix stdout and stderr with color
exec 1> >( perl -ne '$| = 1; print "'"${COLOR}${PREFIX}${RESET}"' | $_"' >&1)
exec 2> >( perl -ne '$| = 1; print "'"${COLOR}${PREFIX}${RESET}"' | $_"' >&2)

exec "$@"
