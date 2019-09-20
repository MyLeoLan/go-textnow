#!/bin/bash

declare -a services=("phonebook" "sms")

for i in "${services[@]}"
do
  protoc -Iapi/proto \
  -I$HOME/go/src \
  -I$HOME/go/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
  --go_out=plugins=grpc:internal/${i} \
  --govalidators_out=internal/${i}  \
  api/proto/${i}.proto
done

for i in "${services[@]}"
do
  protoc -Iapi/proto \
  -I$HOME/go/src \
  -I$HOME/go/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
  --grpc-gateway_out=logtostderr=true:internal/${i} \
  api/proto/${i}.proto
done
