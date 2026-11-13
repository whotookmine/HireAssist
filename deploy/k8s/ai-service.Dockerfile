# The AI Service as an image, for the Kubernetes deployment (ADR-014). The Compose stack runs the
# service from its source and generates the gRPC code each time it starts; this does the same
# steps once, at build time. Built from the repository root, because the contract lives in proto/
# (ADR-007):
#
#   docker build -f deploy/k8s/ai-service.Dockerfile -t hireassist/ai-service:dev .
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

ENV UV_PROJECT_ENVIRONMENT=/opt/venv \
    UV_PYTHON_DOWNLOADS=never \
    UV_LINK_MODE=copy \
    UV_COMPILE_BYTECODE=1
WORKDIR /repo/services/ai-service

# Dependencies first, so this layer stays cached while the code changes.
COPY services/ai-service/pyproject.toml services/ai-service/uv.lock services/ai-service/.python-version ./
RUN uv sync --frozen --no-install-project

# Generate the gRPC code from the contract, then drop the development tools that needed it.
COPY proto /repo/proto
COPY services/ai-service/src ./src
RUN mkdir -p src/ai_service/gen && touch src/ai_service/gen/__init__.py && \
    uv run --frozen python -m grpc_tools.protoc -I../../proto \
      --python_out=src/ai_service/gen --pyi_out=src/ai_service/gen --grpc_python_out=src/ai_service/gen \
      ../../proto/hireassist/ai/v1/ai_service.proto && \
    find src/ai_service/gen -name '*_pb2_grpc.py' -exec sed -i 's/^from hireassist/from ai_service.gen.hireassist/' {} + && \
    uv sync --frozen --no-dev

EXPOSE 50051
USER nobody
CMD ["/opt/venv/bin/python", "-m", "ai_service"]
