FROM node:22-bookworm
RUN npm install --global opencode-ai@1.18.21
RUN mkdir -p /work && chown node:node /work && mkdir -p /root/.multica/profiles/security-recovery
COPY multica /usr/local/bin/multica
COPY opencode-pilot /usr/local/bin/opencode-pilot
RUN chmod 755 /usr/local/bin/opencode-pilot /usr/local/bin/multica && chmod 700 /root/.multica /root/.multica/profiles /root/.multica/profiles/security-recovery
WORKDIR /work
