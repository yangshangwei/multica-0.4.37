FROM node:22-bookworm

RUN install -d -m 0755 /work /work/tasks /root/.codex /root/.codex/multica-sessions \
    /home/node/.codex /work/simulation /usr/local/lib/multica-simulation \
    && install -d -o node -g node -m 0755 /work/lab /work/simulation/audit \
    && install -d -m 0700 /root/.multica /root/.multica/profiles \
    && chmod 0711 /root
COPY multica /usr/local/bin/multica
COPY provider.py /usr/local/lib/multica-simulation/provider.py
COPY codex-scripted-simulation /usr/local/bin/codex-scripted-simulation
RUN chmod 0755 /usr/local/bin/multica /usr/local/bin/codex-scripted-simulation

WORKDIR /work
CMD ["sleep", "infinity"]
