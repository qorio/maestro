#!/bin/bash

# Git commit hash / message
GIT_COMMIT_HASH=$(git rev-list --max-count=1 --reverse HEAD)
GIT_COMMIT_MESSAGE=$(git log -1 | tail -1 | sed -e "s/^[ ]*//g")
BUILD_TIMESTAMP=$(date +"%Y-%m-%d-%H:%M")

SRC_FILE=$1

echo "Git commit $GIT_COMMIT_HASH ($GIT_COMMIT_MESSAGE) on $BUILD_TIMESTAMP"
sed -ri "s/@@GIT_COMMIT_HASH@@/${GIT_COMMIT_HASH}/g" $SRC_FILE
sed -ri "s/@@GIT_COMMIT_MESSAGE@@/${GIT_COMMIT_MESSAGE}/g" $SRC_FILE
sed -ri "s/@@BUILD_TIMESTAMP@@/${BUILD_TIMESTAMP}/g" $SRC_FILE
sed -ri "s/@@BUILD_NUMBER@@/${CIRCLE_BUILD_NUM}/g" $SRC_FILE
