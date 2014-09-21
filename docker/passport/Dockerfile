# Passport Dockerfile
#

# Pull base image.
FROM ubuntu:14.04

# Install passport from build directory
# This assumes that the binary has been copied into the directory where this Dockerfile lives.
COPY passport /root/

RUN chmod a+x /root/passport

# Define mountable directories.
VOLUME ["/data", "/config"]

# Define working directory.
WORKDIR /root

# Define default command.
CMD ["/root/passport", "--logtostderr", "--auth_public_key_file=/config/testAuthKey.pub", "--mongo_hosts=mongodb"]

# Expose ports.

EXPOSE 6060
EXPOSE 7070