FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates git && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install nanobot from local source
COPY pyproject.toml README.md LICENSE ./
RUN mkdir -p nanobot && touch nanobot/__init__.py && \
    uv pip install --system --no-cache . && \
    rm -rf nanobot

COPY nanobot/ nanobot/
RUN uv pip install --system --no-cache .

# Create data directories
RUN mkdir -p /root/.nanobot /data/workspace/robots/default/memory \
    /data/workspace/robots/proactive/memory

EXPOSE 8081

ENTRYPOINT ["nanobot"]
CMD ["gateway"]
