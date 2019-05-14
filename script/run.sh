#!/bin/bash

# SIGTERM or SIGINT trapped (likely SIGTERM from docker), pass it onto app process
function _term_or_init {
  echo "run: caught term or int, telling children to shut down.."
  kill -TERM "$APP_PID" 2>/dev/null
  wait $APP_PID
}

# The bugsnag notifier monitor process needs at least 300ms, in order to ensure that it can send its notify
function _exit {
  echo "run: app exit, wait 1 for monitor cleanup"
  sleep 1
}

trap _term_or_init SIGTERM SIGINT
trap _exit EXIT

./ovh-ip-updater-go &

# Wait on the app process to ensure that this script is able to trap the SIGTERM signal
APP_PID=$!
wait $APP_PID