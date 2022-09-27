#!/bin/bash

set -euo pipefail

source versions.env

VARIABLES=($(IFS=$'\n'; grep '=' versions.env | awk -F= '{print $1}'))
VAR_COUNT=${#VARIABLES[@]}

failures=0
for ci_file in $(ls .github/workflows/*.y*ml); do
  defined_vars=$(grep '^env:.*$' -A "${VAR_COUNT}" "${ci_file}" || true)
  for var_name in $VARIABLES; do
    if echo "${defined_vars}" |grep -q "${var_name}"; then
      value=${!var_name}
      ci_value=$(grep '^env:.*$' -A "${VAR_COUNT}" "${ci_file}" |grep "${var_name}:" |awk '{print $2}')
      if [ "${value}" != "${ci_value}" ]; then
        failures=$(( failures++ ))
        echo "Fix ${ci_file} workflow environment variables to set ${var_name}=${value} (instead of ${ci_value})"
      fi
    fi
  done
done

if (( $failures > 0 )); then
  echo "Found ${failures} version mistmatchs between CI settings and main version defintion file versions.env"
  exit 1 
fi
echo "OK"
