#!/bin/bash

LOCAL_DIR="/home/yejibing/code/ml/dataset/arXiv/output/src"
REMOTE_USER="root"
REMOTE_HOST="server2"
REMOTE_DIR="/home/yejibing/dataset/arXiv"
AGE_IN_MINUTES=1

find "$LOCAL_DIR" -type f -mmin +$AGE_IN_MINUTES -print0 | while IFS= read -r -d '' file; do
    scp "$file" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}"
    if [ $? -eq 0 ]; then
        rm "$file"
        echo "Transferred and removed: $file"
    else
        echo "Failed to transfer: $file"
    fi
done
