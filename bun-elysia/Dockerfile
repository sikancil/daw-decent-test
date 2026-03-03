FROM oven/bun:alpine

WORKDIR /app

# Cache dependencies
COPY package.json bun.lockb* ./
RUN bun install --production

# Copy source code
COPY index.ts .

EXPOSE 3000

# Run directly with Bun
CMD ["bun", "run", "index.ts"]
