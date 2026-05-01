#!/usr/bin/env bash

# Path to your JSON file
FILE="autograder.config.json"

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "⚠️  WARNING: 'jq' is not installed. Skipping total timeout and points calculation."
else
    # Define the jq logic in a variable for readability
    JQ_SCRIPT='
    # Custom function to parse Go time.Duration strings (e.g., "1h30m", "500ms", "10s") into total seconds
    def parse_go_duration:
      [
        # Regex scans for pairs of (Number)(Unit)
        scan("([0-9.]+)([a-zA-Zµ]+)") | 
        { num: (.[0] | tonumber), unit: .[1] } |
        
        # Convert every unit into seconds
        if   .unit == "h"  then .num * 3600
        elif .unit == "m"  then .num * 60
        elif .unit == "s"  then .num
        elif .unit == "ms" then .num / 1000
        elif .unit == "us" or .unit == "µs" then .num / 1000000
        elif .unit == "ns" then .num / 1000000000
        else 0 end
      ] | add // 0; # Sum all extracted parts, default to 0 if empty

    # Calculate total timeout time
    [
      .tests[] | 
      # Use default "0s" if timeout is missing for a test
      (
        ((.timeout // "0s") | parse_go_duration) * (.count / (.parallelCount // 1) | ceil)
      )
    ] | add
    '
    
    # Get total time in seconds
    TOTAL_SECONDS=$(jq -r "$JQ_SCRIPT" "$FILE")

    # Convert seconds to minutes using awk
    TOTAL_MINUTES=$(awk -v sec="$TOTAL_SECONDS" 'BEGIN { printf "%.2f", sec / 60 }')

    # Get total points (defaulting to 0 if the points key is missing)
    TOTAL_POINTS=$(jq -r '[.tests[] | .points // 0] | add' "$FILE")

    # Print out time and points
    printf "Total test timeout requires: %.2f seconds (%s minutes)\n" "$TOTAL_SECONDS" "$TOTAL_MINUTES"
    printf "Total points available: %g\n" "$TOTAL_POINTS"

    # Check if the total exceeds 35 minutes (2100 seconds)
    if awk -v total="$TOTAL_SECONDS" 'BEGIN { if (total > 2100) exit 0; else exit 1 }'; then
        echo "⚠️  WARNING: Total timeout exceeds 35 minutes! This may be too high."
    fi
fi

echo "----------------------------------------"

# Remove existing zip file, warning before deletion
if [ -e "autograder.zip" ]; then
    echo "⚠️  WARNING: autograder.zip already exists. It will be replaced."
    rm -f autograder.zip
fi

AUTOGRADER_FILES="src run_autograder setup.sh"
CONFIG_FILES="autograder.config.json replacement_files custom_setup.sh custom_run_autograder.sh required_files.txt"

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
pushd "$TEMP_DIR" > /dev/null
zip -r "$DST_DIR/autograder.zip" * -x "*.DS_Store"
popd > /dev/null

rm -rf "$TEMP_DIR"