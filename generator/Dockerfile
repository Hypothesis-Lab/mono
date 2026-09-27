FROM ghcr.io/astral-sh/uv@sha256:c0ba49559fc5622531fd05a5747b52afb49ffa883574bbf8eb719ebd103efb84 AS builder

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project --extra evaluation

COPY app/ app/
COPY alembic.ini ./
COPY migrations/ migrations/
RUN uv sync --frozen --no-dev --extra evaluation


FROM ghcr.io/astral-sh/uv@sha256:c0ba49559fc5622531fd05a5747b52afb49ffa883574bbf8eb719ebd103efb84

WORKDIR /app

COPY --from=builder /app/.venv /app/.venv
COPY app/ app/
COPY alembic.ini ./
COPY migrations/ migrations/

ENV PATH="/app/.venv/bin:$PATH"

CMD ["python", "-m", "app.cmd.server"]
