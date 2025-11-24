import { chromium, firefox, webkit, Browser, BrowserContext, Page } from 'playwright';
import { v4 as uuidv4 } from 'uuid';
import { Logger } from 'winston';

export interface SessionConfig {
  browserType: 'chromium' | 'firefox' | 'webkit';
  headless: boolean;
  viewportWidth: number;
  viewportHeight: number;
  userAgent?: string;
  extraHttpHeaders?: Record<string, string>;
  ignoreHttpsErrors?: boolean;
  proxyServer?: string;
  timeoutMs?: number;
}

export interface CapturedRequest {
  requestId: string;
  method: string;
  url: string;
  headers: Record<string, string>;
  body?: string;
  resourceType: string;
  timestamp: number;
  response?: {
    statusCode: number;
    statusText: string;
    headers: Record<string, string>;
    body?: string;
    timestamp: number;
  };
}

export interface Session {
  id: string;
  browser: Browser;
  context: BrowserContext;
  page: Page;
  createdAt: number;
  lastActivity: number;
  capturedRequests: CapturedRequest[];
  isCapturing: boolean;
  captureConfig?: {
    urlPatterns: string[];
    captureRequestBody: boolean;
    captureResponseBody: boolean;
  };
}

export class SessionManager {
  private sessions: Map<string, Session> = new Map();
  private logger: Logger;

  constructor(logger: Logger) {
    this.logger = logger;
  }

  async createSession(config: SessionConfig): Promise<string> {
    const sessionId = uuidv4();
    this.logger.info('Creating session', { sessionId, browserType: config.browserType });

    // Select browser type
    let browserType;
    switch (config.browserType) {
      case 'firefox':
        browserType = firefox;
        break;
      case 'webkit':
        browserType = webkit;
        break;
      default:
        browserType = chromium;
    }

    // Launch browser
    const browser = await browserType.launch({
      headless: config.headless !== false,
      proxy: config.proxyServer ? { server: config.proxyServer } : undefined
    });

    // Create context
    const context = await browser.newContext({
      viewport: {
        width: config.viewportWidth || 1920,
        height: config.viewportHeight || 1080
      },
      userAgent: config.userAgent,
      extraHTTPHeaders: config.extraHttpHeaders,
      ignoreHTTPSErrors: config.ignoreHttpsErrors
    });

    // Set default timeout
    if (config.timeoutMs) {
      context.setDefaultTimeout(config.timeoutMs);
    }

    // Create page
    const page = await context.newPage();

    const session: Session = {
      id: sessionId,
      browser,
      context,
      page,
      createdAt: Date.now(),
      lastActivity: Date.now(),
      capturedRequests: [],
      isCapturing: false
    };

    this.sessions.set(sessionId, session);
    this.logger.info('Session created', { sessionId });

    return sessionId;
  }

  getSession(sessionId: string): Session | undefined {
    const session = this.sessions.get(sessionId);
    if (session) {
      session.lastActivity = Date.now();
    }
    return session;
  }

  async destroySession(sessionId: string): Promise<boolean> {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return false;
    }

    this.logger.info('Destroying session', { sessionId });

    try {
      await session.context.close();
      await session.browser.close();
    } catch (err) {
      this.logger.warn('Error closing browser', { sessionId, error: err });
    }

    this.sessions.delete(sessionId);
    return true;
  }

  async destroyAll(): Promise<void> {
    this.logger.info('Destroying all sessions', { count: this.sessions.size });
    const sessionIds = Array.from(this.sessions.keys());
    await Promise.all(sessionIds.map(id => this.destroySession(id)));
  }

  async startNetworkCapture(
    sessionId: string,
    urlPatterns: string[],
    captureRequestBody: boolean,
    captureResponseBody: boolean
  ): Promise<boolean> {
    const session = this.getSession(sessionId);
    if (!session) return false;

    session.capturedRequests = [];
    session.isCapturing = true;
    session.captureConfig = {
      urlPatterns,
      captureRequestBody,
      captureResponseBody
    };

    const pendingRequests = new Map<string, CapturedRequest>();

    // Listen for requests
    session.page.on('request', async (request) => {
      if (!session.isCapturing) return;

      // Check URL patterns
      if (urlPatterns.length > 0) {
        const matches = urlPatterns.some(pattern => {
          const regex = new RegExp(pattern.replace(/\*/g, '.*'));
          return regex.test(request.url());
        });
        if (!matches) return;
      }

      const requestId = uuidv4();
      const captured: CapturedRequest = {
        requestId,
        method: request.method(),
        url: request.url(),
        headers: request.headers(),
        resourceType: request.resourceType(),
        timestamp: Date.now()
      };

      if (captureRequestBody) {
        try {
          captured.body = request.postData() || undefined;
        } catch {
          // Ignore errors
        }
      }

      pendingRequests.set(request.url() + request.method(), captured);
    });

    // Listen for responses
    session.page.on('response', async (response) => {
      if (!session.isCapturing) return;

      const request = response.request();
      const key = request.url() + request.method();
      const captured = pendingRequests.get(key);

      if (captured) {
        captured.response = {
          statusCode: response.status(),
          statusText: response.statusText(),
          headers: response.headers(),
          timestamp: Date.now()
        };

        if (captureResponseBody) {
          try {
            captured.response.body = await response.text();
          } catch {
            // Ignore errors (binary content, etc.)
          }
        }

        session.capturedRequests.push(captured);
        pendingRequests.delete(key);
      }
    });

    this.logger.info('Network capture started', { sessionId });
    return true;
  }

  stopNetworkCapture(sessionId: string): boolean {
    const session = this.getSession(sessionId);
    if (!session) return false;

    session.isCapturing = false;
    this.logger.info('Network capture stopped', { sessionId, requestCount: session.capturedRequests.length });
    return true;
  }

  getCapturedRequests(sessionId: string, clearAfter: boolean): CapturedRequest[] {
    const session = this.getSession(sessionId);
    if (!session) return [];

    const requests = [...session.capturedRequests];
    if (clearAfter) {
      session.capturedRequests = [];
    }
    return requests;
  }
}
