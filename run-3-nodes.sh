#!/usr/bin/env bash
set -euo pipefail

PORT=3030 P2P_PORT=4030 DATA_DIR=./node-a PEERS="" go run . &
PORT=3031 P2P_PORT=4031 DATA_DIR=./node-b PEERS="127.0.0.1:4030" go run . &
PORT=3032 P2P_PORT=4032 DATA_DIR=./node-c PEERS="127.0.0.1:4030,127.0.0.1:4031" go run . &
wait
