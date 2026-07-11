#!/bin/bash

set -e

tokill=$$

runtimeConfig="./config.runtime.json"
sourceConfig="./config.json"

runSingbox(){
  configFile="$sourceConfig"
  if [ -f "$runtimeConfig" ]; then
    configFile="$runtimeConfig"
  fi
  ./sing-box run -c "$configFile" &
  tokill=$!
}

terminateSingbox()
{
  if kill -0 "$tokill" > /dev/null 2>&1; then
    echo "Terminating sing-box PID=$tokill"
    kill "$tokill"
    waited=0
    while kill -0 "$tokill" > /dev/null 2>&1; do
      sleep 1
      waited=$((waited + 1))
      if [ "$waited" -ge 10 ]; then
        echo "sing-box did not stop gracefully; killing PID=$tokill"
        kill -KILL "$tokill" > /dev/null 2>&1 || true
        break
      fi
    done
  fi
}

restartSingbox()
{
  terminateSingbox
  runSingbox
}

trap terminateSingbox SIGINT SIGTERM
trap restartSingbox SIGHUP

runSingbox

while true
do
    sleep 5
    signal=""
    if [ -f "signal" ]; then
        signal=$(cat signal)
        echo "Signal received: $signal"
        rm -f signal > /dev/null 2>&1
        case ${signal} in
            "stop")
                terminateSingbox
                ;;
            "restart")
                restartSingbox
                ;;
        esac
    fi

    if ! kill -0 "$tokill" > /dev/null 2>&1; then
        if [ "$signal" != "stop" ]; then
            echo "sing-box with PID $tokill crashed. Breaking the loop..."
            exit 1
        fi
    fi
done
