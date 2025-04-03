#!/usr/bin/env bash

# Remove existing zip file, warning before deletion
if [ -e "autograder.zip" ]; then
    echo "Warning: autograder.zip already exists. It will be replaced."
    rm -f autograder.zip
fi

AUTOGRADER_FILES="src run_autograder setup.sh"
CONFIG_FILES="autograder.config.json replacement_files custom_setup.sh custom_run_autograder.sh"

AUTOGRADER_FOLDER=$(dirname -- "$( readlink -f -- "$0"; )";)
CONFIG_FOLDER=$(dirname "$AUTOGRADER_FOLDER")

# Create a temporary directory for zip operations
TEMP_DIR=$(mktemp -d)

# Copy autograder files
for file in $AUTOGRADER_FILES; do
    if [ -e "$AUTOGRADER_FOLDER/$file" ]; then
        cp -R "$AUTOGRADER_FOLDER/$file" "$TEMP_DIR/"
    fi
done

# Copy config files
for file in $CONFIG_FILES; do
    if [ -e "$CONFIG_FOLDER/$file" ]; then
        cp -R "$CONFIG_FOLDER/$file" "$TEMP_DIR/"
    fi
done

# Create zip from the temporary directory
DST_DIR=$(pwd)
pushd "$TEMP_DIR"
zip -r "$DST_DIR/autograder.zip" * -x "*.DS_Store"
popd

rm -rf "$TEMP_DIR"