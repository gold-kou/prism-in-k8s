#!/bin/sh

if [ ! -s /app/openapi.yaml ]; then
  echo "openapi.yaml is empty or does not exist. Copying openapi-sample.yaml to openapi.yaml."
  cp /app/openapi-sample.yaml /app/openapi.yaml
else
  echo "openapi.yaml is not empty."
fi
