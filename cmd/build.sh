#!/bin/bash

for dir in */; do
  if [ -d "$dir" ]; then
    echo "Building in folder $dir..."
    pushd "$dir" > /dev/null
    go build .
    popd > /dev/null
  fi
done

echo "All builds complete."
