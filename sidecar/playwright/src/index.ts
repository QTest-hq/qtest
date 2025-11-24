import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import path from 'path';
import { createLogger, format, transports } from 'winston';
import { SessionManager } from './session-manager';
import { createHandlers } from './handlers';

const logger = createLogger({
  level: process.env.LOG_LEVEL || 'info',
  format: format.combine(
    format.timestamp(),
    format.json()
  ),
  transports: [
    new transports.Console()
  ]
});

const PROTO_PATH = path.join(__dirname, '../proto/playwright.proto');
const PORT = process.env.GRPC_PORT || '50051';
const HOST = process.env.GRPC_HOST || '0.0.0.0';

async function main() {
  logger.info('Starting Playwright sidecar service...');

  // Load proto definition
  const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true
  });

  const protoDescriptor = grpc.loadPackageDefinition(packageDefinition) as any;
  const playwrightProto = protoDescriptor.playwright;

  // Create session manager
  const sessionManager = new SessionManager(logger);

  // Create gRPC server
  const server = new grpc.Server();

  // Add service handlers
  const handlers = createHandlers(sessionManager, logger);
  server.addService(playwrightProto.PlaywrightService.service, handlers);

  // Start server
  const address = `${HOST}:${PORT}`;
  server.bindAsync(
    address,
    grpc.ServerCredentials.createInsecure(),
    (err, port) => {
      if (err) {
        logger.error('Failed to bind server', { error: err.message });
        process.exit(1);
      }
      logger.info(`Playwright sidecar listening on ${address}`);
    }
  );

  // Graceful shutdown
  const shutdown = async () => {
    logger.info('Shutting down...');
    await sessionManager.destroyAll();
    server.forceShutdown();
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
}

main().catch((err) => {
  console.error('Fatal error:', err);
  process.exit(1);
});
