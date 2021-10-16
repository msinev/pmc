#!/bin/bash
until [[ $fileprocess == "." ]]
  do
    date
    if [[ $fileprocess ]];then
      echo "Processing $fileprocess"
      sleep 1
    else
      echo "Wait to start"
      sleep 1
    fi
  fileprocess=$(curl -s "http://192.168.1.199:3000/get")
  done
date
echo "Complete!"