#!/bin/bash

set -euxo pipefail

src=$1
dest=$2

# If dest is a local directory, download the model to that directory.
# Otherwise, download to a temporary directory and upload from there.
dest_type=""
if [[ -d $dest ]]; then
    dir=$dest
    dest_type="dir"
else
    dir=$(mktemp -d)
    dest_type="url"
fi

# Download
case $src in
    "hf://"*)
        repo=${src#hf://}
        huggingface-cli download --local-dir $dir $repo
        rm -rf $dir/.cache
        ;;
    "s3://"*)
        aws s3 sync $src $dir
        ;;
    "gs://"*)
        gcloud storage rsync $src $dir
        ;;
    "oss://"*)
        ossutil sync $src $dir
        ;;
    *)
        echo "Unsupported source url: $src"
        exit 1
        ;;
esac

# Upload
if [[ $dest_type == "url" ]]; then
    case $dest in
        "hf://"*)
            repo=${dest#hf://}
            huggingface-cli upload $repo $dir
            ;;
        "s3://"*)
            aws s3 sync $dir $dest
            ;;
        "gs://"*)
            gcloud storage rsync $dir $dest
            ;;
        "oss://"*)
            ossutil sync $dir $dest
            ;;
        *)
            echo "Unsupported destination url: $dest"
            exit 1
            ;;
    esac
fi