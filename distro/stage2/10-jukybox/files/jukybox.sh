#!/bin/bash

while true; do
  ./jukybox
  if [ $? -eq 123 ]; then
    /sbin/shutdown -h now
  fi
done
