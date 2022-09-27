#!/bin/bash

set -euo pipefail

source versions.env

VARIABLES=($(IFS=$'\n'; grep '=' versions.env | awk -F= '{print $1}'))
VAR_COUNT=${#VARIABLES[@]}

failures=0
for ci_file in $(ls .github/workflows/*.y*ml); do
  # Check variable name definitions
  defined_vars=$(grep '^env:.*$' -A "${VAR_COUNT}" "${ci_file}" || true)
  for var_name in $VARIABLES; do
    if echo "${defined_vars}" |grep -q "${var_name}"; then
      value=${!var_name}
      ci_value=$(grep '^env:.*$' -A "${VAR_COUNT}" "${ci_file}" |grep "${var_name}:" |awk '{print $2}')
      if [ "${value}" != "${ci_value}" ]; then
        ((failures+=1))
        echo "Fix ${ci_file} workflow environment variables to set ${var_name}=${value} (instead of ${ci_value})"
      fi
    fi
  done

  # Check static matrices
  expected_matrix_regex="go: \[\""${GO_VERSION}"\", \""${GO_NEXT_VERSION}"\"\]"
  expected_matrix=$(echo "${expected_matrix_regex}" | sed -e 's/\\\[/[/' -e 's/\\\]/]/')
  if grep 'matrix:' -A3 "${ci_file}" |grep -s 'go:' > /dev/null; then
    matrices=($(IFS=$'\n'; grep 'matrix:' -A3 "${ci_file}" |grep 'go:' || true))
    matrices_count=${#matrices[@]}
    if (( $matrices_count > 0 )); then
      for matrix in $matrices; do
        if ! echo "${matrix}" | grep -q "${expected_matrix_regex}"; then
          ((failures+=1))
          echo "Fix ${ci_file} workflow matrix to be '${expected_matrix}' instead of '${matrix}'"
        fi
      done
    fi
  fi
done

if (( $failures > 0 )); then
  echo "Found ${failures} version mistmatchs between CI settings and main version definition file: versions.env"
  exit 1 
fi
echo "OK failures=${failures}"
