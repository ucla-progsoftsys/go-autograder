# Used to test locally. Not used in the actual autograder

# Use the Gradescope autograder base image with Ubuntu 22.04, specifying AMD64 architecture
FROM --platform=linux/amd64 gradescope/autograder-base:ubuntu-22.04

# Create the source directory for the autograder
RUN mkdir -p /autograder/source

# Copy all files from the build context to the autograder source directory
COPY ./go-autograder /autograder/source
COPY ./autograder.config.json /autograder/source/autograder.config.json
COPY ./replacement_files /autograder/source/replacement_files
COPY ./required_files.txt /autograder/source/required_files.txt
COPY ./custom_setup.sh /autograder/source/custom_setup.sh
COPY ./custom_run_autograder.sh /autograder/source/custom_run_autograder.sh

# Copy the run_autograder script to the required location
RUN cp /autograder/source/run_autograder /autograder/run_autograder

# Convert Windows line endings to Unix format to prevent execution errors
RUN dos2unix /autograder/run_autograder /autograder/source/setup.sh

# Make the run_autograder script executable
RUN chmod +x /autograder/run_autograder

# Run the setup script to configure the autograder environment
RUN bash /gradescope/setup.sh