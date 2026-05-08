$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path

Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$root'; `$env:PORT='3030'; `$env:P2P_PORT='4030'; `$env:DATA_DIR='./node-a'; `$env:PEERS=''; go run ."
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$root'; `$env:PORT='3031'; `$env:P2P_PORT='4031'; `$env:DATA_DIR='./node-b'; `$env:PEERS='127.0.0.1:4030'; go run ."
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$root'; `$env:PORT='3032'; `$env:P2P_PORT='4032'; `$env:DATA_DIR='./node-c'; `$env:PEERS='127.0.0.1:4030,127.0.0.1:4031'; go run ."
