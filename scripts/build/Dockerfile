FROM goreleaser/goreleaser:v2.8.2

RUN apk add --no-cache \
		ca-certificates \
		git \
		gnupg

ENV GPG_TTY /dev/console
ARG USER_ID=1000
ARG GROUP_ID=1000

RUN addgroup --gid "$GROUP_ID" user && \
    adduser -D -u "$USER_ID" -G user -h /home/user -s /bin/bash user && \
    mkdir -p /home/user/.gnupg && \
    chown "${USER_ID}:${GROUP_ID}" /home/user/.gnupg && \
    chmod 700 /home/user/.gnupg

COPY build.sh /usr/local/bin/

# Switch to non-root user
USER user

COPY --chown=user:user peter-junos.pub /home/user/
RUN gpg --batch --import /home/user/peter-junos.pub

WORKDIR /home/user/app

ENTRYPOINT ["/usr/local/bin/build.sh"]
