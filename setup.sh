#!/usr/bin/env bash

GO_VERSION="1.22.12"

# Check if custom_setup.sh exists and source it
if [ -f /autograder/source/custom_setup.sh ]; then
    source /autograder/source/custom_setup.sh
fi

apt-get update && apt-get install -y wget rsync
# Define Go version to install

# Install Go
echo "Installing Go version $GO_VERSION..."
wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -q -O /tmp/go.tar.gz
tar -C /usr/local -xzf /tmp/go.tar.gz
ln -s /usr/local/go/bin/go /usr/bin/go

# Create a user to run the student code in
adduser student --disabled-password --gecos ""

# Restrict permissions
mkdir -p /autograder/results
chmod 700 /autograder/results

mkdir -p /autograder/source
chmod -R 755 /autograder/source