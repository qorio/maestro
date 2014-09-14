# Passport Dockerfile
#

# Pull base image.
FROM ubuntu:14.04

# Install passport from build directory
# This assumes that the binary has been copied into the directory where this Dockerfile lives.
COPY passport /root/
COPY testAuthKey.pub /root/testAuthKey.pub
RUN chmod a+x /root/passport

# Define mountable directories.
VOLUME ["/data", "/config"]

# Define working directory.
WORKDIR /root

# Define default command.
CMD ["/root/passport", "--logtostderr", "--dir=/data", "--auth_public_key_file=/config/testAuthKey.pub"]

# Expose ports.

EXPOSE 5050
EXPOSE 7070