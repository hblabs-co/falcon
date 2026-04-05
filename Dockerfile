FROM qdrant/qdrant:latest AS qdrant
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
