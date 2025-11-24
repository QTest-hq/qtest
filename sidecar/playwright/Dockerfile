FROM mcr.microsoft.com/playwright:v1.40.0-focal

WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm ci --only=production

# Copy source
COPY . .

# Build TypeScript
RUN npm run build

# Expose gRPC port
EXPOSE 50051

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD node -e "require('@grpc/grpc-js').credentials.createInsecure()" || exit 1

# Run the service
CMD ["node", "dist/index.js"]
