import { SessionManager, SessionConfig } from './session-manager';
import { createHandlers } from './handlers';
import { createLogger, format, transports } from 'winston';

// Mock logger for tests
const logger = createLogger({
  level: 'error',
  format: format.simple(),
  transports: [new transports.Console()]
});

describe('SessionManager', () => {
  let sessionManager: SessionManager;

  beforeEach(() => {
    sessionManager = new SessionManager(logger);
  });

  afterEach(async () => {
    await sessionManager.destroyAll();
  });

  describe('createSession', () => {
    it('should create a chromium session', async () => {
      const sessionId = await sessionManager.createSession({
        browserType: 'chromium',
        headless: true,
        viewportWidth: 1920,
        viewportHeight: 1080
      });

      expect(sessionId).toBeDefined();
      expect(typeof sessionId).toBe('string');
      expect(sessionId.length).toBeGreaterThan(0);
    });

    it('should create session with custom viewport', async () => {
      const sessionId = await sessionManager.createSession({
        browserType: 'chromium',
        headless: true,
        viewportWidth: 800,
        viewportHeight: 600
      });

      const session = sessionManager.getSession(sessionId);
      expect(session).toBeDefined();
    });
  });

  describe('getSession', () => {
    it('should return session when exists', async () => {
      const sessionId = await sessionManager.createSession({
        browserType: 'chromium',
        headless: true,
        viewportWidth: 1920,
        viewportHeight: 1080
      });

      const session = sessionManager.getSession(sessionId);
      expect(session).toBeDefined();
      expect(session?.id).toBe(sessionId);
    });

    it('should return undefined for non-existent session', () => {
      const session = sessionManager.getSession('non-existent-id');
      expect(session).toBeUndefined();
    });
  });

  describe('destroySession', () => {
    it('should destroy existing session', async () => {
      const sessionId = await sessionManager.createSession({
        browserType: 'chromium',
        headless: true,
        viewportWidth: 1920,
        viewportHeight: 1080
      });

      const result = await sessionManager.destroySession(sessionId);
      expect(result).toBe(true);

      const session = sessionManager.getSession(sessionId);
      expect(session).toBeUndefined();
    });

    it('should return false for non-existent session', async () => {
      const result = await sessionManager.destroySession('non-existent-id');
      expect(result).toBe(false);
    });
  });

  describe('network capture', () => {
    it('should start and stop network capture', async () => {
      const sessionId = await sessionManager.createSession({
        browserType: 'chromium',
        headless: true,
        viewportWidth: 1920,
        viewportHeight: 1080
      });

      const startResult = await sessionManager.startNetworkCapture(
        sessionId,
        [],
        true,
        true
      );
      expect(startResult).toBe(true);

      const stopResult = sessionManager.stopNetworkCapture(sessionId);
      expect(stopResult).toBe(true);
    });

    it('should return false for non-existent session', async () => {
      const result = await sessionManager.startNetworkCapture(
        'non-existent',
        [],
        true,
        true
      );
      expect(result).toBe(false);
    });
  });
});

describe('Handlers', () => {
  let sessionManager: SessionManager;
  let handlers: ReturnType<typeof createHandlers>;

  beforeEach(() => {
    sessionManager = new SessionManager(logger);
    handlers = createHandlers(sessionManager, logger);
  });

  afterEach(async () => {
    await sessionManager.destroyAll();
  });

  describe('CreateSession', () => {
    it('should create session via handler', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        (err: any, response: any) => {
          expect(err).toBeNull();
          expect(response.success).toBe(true);
          expect(response.session_id).toBeDefined();
          done();
        }
      );
    });
  });

  describe('DestroySession', () => {
    it('should destroy session via handler', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        (err: any, createResponse: any) => {
          handlers.DestroySession(
            { request: { session_id: createResponse.session_id } },
            (err2: any, destroyResponse: any) => {
              expect(err2).toBeNull();
              expect(destroyResponse.success).toBe(true);
              done();
            }
          );
        }
      );
    });
  });

  describe('Navigate', () => {
    it('should navigate to URL', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        async (err: any, createResponse: any) => {
          handlers.Navigate(
            {
              request: {
                session_id: createResponse.session_id,
                url: 'data:text/html,<html><body><h1>Test</h1></body></html>',
                timeout_ms: 5000
              }
            },
            (err2: any, navResponse: any) => {
              expect(err2).toBeNull();
              expect(navResponse.success).toBe(true);
              done();
            }
          );
        }
      );
    });

    it('should fail for non-existent session', (done) => {
      handlers.Navigate(
        {
          request: {
            session_id: 'non-existent',
            url: 'https://example.com'
          }
        },
        (err: any, response: any) => {
          expect(response.success).toBe(false);
          expect(response.error).toContain('Session not found');
          done();
        }
      );
    });
  });

  describe('GetDOMSnapshot', () => {
    it('should get DOM snapshot', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        (err: any, createResponse: any) => {
          handlers.Navigate(
            {
              request: {
                session_id: createResponse.session_id,
                url: 'data:text/html,<html><body><div id="test">Hello</div></body></html>'
              }
            },
            () => {
              handlers.GetDOMSnapshot(
                {
                  request: {
                    session_id: createResponse.session_id,
                    include_attributes: true,
                    max_depth: 5
                  }
                },
                (err2: any, snapResponse: any) => {
                  expect(err2).toBeNull();
                  expect(snapResponse.success).toBe(true);
                  expect(snapResponse.html).toContain('Hello');
                  done();
                }
              );
            }
          );
        }
      );
    });
  });

  describe('TakeScreenshot', () => {
    it('should take screenshot', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        (err: any, createResponse: any) => {
          handlers.Navigate(
            {
              request: {
                session_id: createResponse.session_id,
                url: 'data:text/html,<html><body><h1>Screenshot Test</h1></body></html>'
              }
            },
            () => {
              handlers.TakeScreenshot(
                {
                  request: {
                    session_id: createResponse.session_id,
                    full_page: false,
                    format: 'png'
                  }
                },
                (err2: any, ssResponse: any) => {
                  expect(err2).toBeNull();
                  expect(ssResponse.success).toBe(true);
                  expect(ssResponse.image_data).toBeDefined();
                  expect(ssResponse.image_data.length).toBeGreaterThan(0);
                  done();
                }
              );
            }
          );
        }
      );
    });
  });

  describe('GetPageLinks', () => {
    it('should extract page links', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        (err: any, createResponse: any) => {
          handlers.Navigate(
            {
              request: {
                session_id: createResponse.session_id,
                url: 'data:text/html,<html><body><a href="/page1">Link 1</a><a href="https://external.com">External</a></body></html>'
              }
            },
            () => {
              handlers.GetPageLinks(
                {
                  request: {
                    session_id: createResponse.session_id,
                    same_origin_only: false
                  }
                },
                (err2: any, linksResponse: any) => {
                  expect(err2).toBeNull();
                  expect(linksResponse.success).toBe(true);
                  expect(linksResponse.links.length).toBe(2);
                  done();
                }
              );
            }
          );
        }
      );
    });
  });

  describe('GetPageForms', () => {
    it('should extract page forms', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        (err: any, createResponse: any) => {
          handlers.Navigate(
            {
              request: {
                session_id: createResponse.session_id,
                url: `data:text/html,<html><body>
                  <form action="/login" method="post">
                    <input type="text" name="username" placeholder="Username">
                    <input type="password" name="password" placeholder="Password">
                    <button type="submit">Login</button>
                  </form>
                </body></html>`
              }
            },
            () => {
              handlers.GetPageForms(
                {
                  request: {
                    session_id: createResponse.session_id
                  }
                },
                (err2: any, formsResponse: any) => {
                  expect(err2).toBeNull();
                  expect(formsResponse.success).toBe(true);
                  expect(formsResponse.forms.length).toBe(1);
                  expect(formsResponse.forms[0].method).toBe('post');
                  expect(formsResponse.forms[0].fields.length).toBe(2);
                  done();
                }
              );
            }
          );
        }
      );
    });
  });

  describe('Click and Fill', () => {
    it('should click and fill elements', (done) => {
      handlers.CreateSession(
        { request: { browser_type: 'chromium', headless: true } },
        (err: any, createResponse: any) => {
          handlers.Navigate(
            {
              request: {
                session_id: createResponse.session_id,
                url: `data:text/html,<html><body>
                  <input type="text" id="input1">
                  <button id="btn1" onclick="document.getElementById('input1').value='clicked'">Click Me</button>
                </body></html>`
              }
            },
            () => {
              handlers.Fill(
                {
                  request: {
                    session_id: createResponse.session_id,
                    selector: '#input1',
                    value: 'test value'
                  }
                },
                (fillErr: any, fillResponse: any) => {
                  expect(fillErr).toBeNull();
                  expect(fillResponse.success).toBe(true);

                  handlers.Click(
                    {
                      request: {
                        session_id: createResponse.session_id,
                        selector: '#btn1'
                      }
                    },
                    (clickErr: any, clickResponse: any) => {
                      expect(clickErr).toBeNull();
                      expect(clickResponse.success).toBe(true);
                      done();
                    }
                  );
                }
              );
            }
          );
        }
      );
    });
  });
});
