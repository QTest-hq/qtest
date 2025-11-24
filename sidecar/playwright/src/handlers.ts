import { Logger } from 'winston';
import { SessionManager, SessionConfig } from './session-manager';

type GrpcCallback<T> = (error: Error | null, response?: T) => void;

export function createHandlers(sessionManager: SessionManager, logger: Logger) {
  return {
    // Session management
    async CreateSession(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const req = call.request;
        const config: SessionConfig = {
          browserType: req.browser_type || 'chromium',
          headless: req.headless !== false,
          viewportWidth: req.viewport_width || 1920,
          viewportHeight: req.viewport_height || 1080,
          userAgent: req.user_agent,
          extraHttpHeaders: req.extra_http_headers,
          ignoreHttpsErrors: req.ignore_https_errors,
          proxyServer: req.proxy_server,
          timeoutMs: req.timeout_ms
        };

        const sessionId = await sessionManager.createSession(config);
        callback(null, { session_id: sessionId, success: true });
      } catch (err: any) {
        logger.error('CreateSession failed', { error: err.message });
        callback(null, { success: false, error: err.message });
      }
    },

    async DestroySession(
      call: { request: { session_id: string } },
      callback: GrpcCallback<any>
    ) {
      try {
        const success = await sessionManager.destroySession(call.request.session_id);
        callback(null, { success });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async GetSessionStatus(
      call: { request: { session_id: string } },
      callback: GrpcCallback<any>
    ) {
      try {
        const session = sessionManager.getSession(call.request.session_id);
        if (!session) {
          callback(null, { exists: false });
          return;
        }

        callback(null, {
          exists: true,
          current_url: await session.page.url(),
          page_title: await session.page.title(),
          created_at: session.createdAt,
          last_activity: session.lastActivity
        });
      } catch (err: any) {
        callback(null, { exists: false });
      }
    },

    // Navigation
    async Navigate(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, url, timeout_ms, wait_until } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const response = await session.page.goto(url, {
          timeout: timeout_ms || 30000,
          waitUntil: wait_until || 'load'
        });

        callback(null, {
          success: true,
          final_url: session.page.url(),
          status_code: response?.status() || 0
        });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async Click(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, timeout_ms, force, click_count, button } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        await session.page.click(selector, {
          timeout: timeout_ms || 30000,
          force: force || false,
          clickCount: click_count || 1,
          button: button || 'left'
        });

        callback(null, { success: true });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async Fill(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, value, timeout_ms, force } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        await session.page.fill(selector, value, {
          timeout: timeout_ms || 30000,
          force: force || false
        });

        callback(null, { success: true });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async Select(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, values, timeout_ms } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const selected = await session.page.selectOption(selector, values, {
          timeout: timeout_ms || 30000
        });

        callback(null, { success: true, selected_values: selected });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async Press(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, key, timeout_ms } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        await session.page.press(selector, key, {
          timeout: timeout_ms || 30000
        });

        callback(null, { success: true });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async Hover(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, timeout_ms } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        await session.page.hover(selector, {
          timeout: timeout_ms || 30000
        });

        callback(null, { success: true });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async WaitForSelector(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, timeout_ms, state } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        await session.page.waitForSelector(selector, {
          timeout: timeout_ms || 30000,
          state: state || 'visible'
        });

        callback(null, { success: true, found: true });
      } catch (err: any) {
        callback(null, { success: false, error: err.message, found: false });
      }
    },

    async WaitForNavigation(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, timeout_ms, wait_until } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        await session.page.waitForNavigation({
          timeout: timeout_ms || 30000,
          waitUntil: wait_until || 'load'
        });

        callback(null, { success: true, url: session.page.url() });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    // DOM operations
    async GetDOMSnapshot(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, include_styles, include_attributes, max_depth } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const html = await session.page.content();

        // Get DOM tree as JSON
        const domTree = await session.page.evaluate(({ sel, maxDepth, includeAttrs }) => {
          function serializeNode(node: Element, depth: number): any {
            if (depth > maxDepth) return null;

            const result: any = {
              tag_name: node.tagName?.toLowerCase() || '',
              id: node.id || '',
              class_names: Array.from(node.classList || []),
              text_content: node.childNodes.length === 1 && node.childNodes[0].nodeType === 3
                ? node.textContent?.trim() || ''
                : '',
              children: [],
              is_visible: node.checkVisibility?.() ?? true
            };

            if (includeAttrs) {
              result.attributes = {};
              for (const attr of Array.from(node.attributes || [])) {
                result.attributes[attr.name] = attr.value;
              }
            }

            const rect = node.getBoundingClientRect?.();
            if (rect) {
              result.bounding_box = {
                x: rect.x,
                y: rect.y,
                width: rect.width,
                height: rect.height
              };
            }

            for (const child of Array.from(node.children || [])) {
              const serialized = serializeNode(child, depth + 1);
              if (serialized) {
                result.children.push(serialized);
              }
            }

            return result;
          }

          const root = sel ? document.querySelector(sel) : document.documentElement;
          return root ? serializeNode(root, 0) : null;
        }, {
          sel: selector || null,
          maxDepth: max_depth || 10,
          includeAttrs: include_attributes !== false
        });

        callback(null, { success: true, root: domTree, html });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async GetElement(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, timeout_ms } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const element = await session.page.waitForSelector(selector, {
          timeout: timeout_ms || 30000
        });

        if (!element) {
          callback(null, { success: false, error: 'Element not found' });
          return;
        }

        const info = await element.evaluate((el) => {
          const attrs: Record<string, string> = {};
          for (let i = 0; i < el.attributes.length; i++) {
            const attr = el.attributes[i];
            attrs[attr.name] = attr.value;
          }
          return {
            tag_name: el.tagName.toLowerCase(),
            id: el.id,
            class_names: Array.from(el.classList),
            text_content: el.textContent?.trim() || '',
            attributes: attrs
          };
        });

        const box = await element.boundingBox();

        callback(null, {
          success: true,
          element: {
            ...info,
            bounding_box: box ? {
              x: box.x,
              y: box.y,
              width: box.width,
              height: box.height
            } : null,
            is_visible: await element.isVisible()
          }
        });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async GetElements(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const elements = await session.page.$$(selector);
        const results = await Promise.all(
          elements.map(async (el) => {
            const info = await el.evaluate((e) => ({
              tag_name: e.tagName.toLowerCase(),
              id: e.id,
              class_names: Array.from(e.classList),
              text_content: e.textContent?.trim() || ''
            }));
            return { ...info, is_visible: await el.isVisible() };
          })
        );

        callback(null, { success: true, elements: results });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async EvaluateScript(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, script, arg } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const parsedArg = arg ? JSON.parse(arg) : undefined;
        const result = await session.page.evaluate(
          new Function('arg', `return (${script})(arg)`) as any,
          parsedArg
        );

        callback(null, { success: true, result: JSON.stringify(result) });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    // Screenshots
    async TakeScreenshot(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, full_page, format, quality } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const imageData = await session.page.screenshot({
          fullPage: full_page || false,
          type: format === 'jpeg' ? 'jpeg' : 'png',
          quality: format === 'jpeg' ? (quality || 80) : undefined
        });

        callback(null, {
          success: true,
          image_data: imageData,
          format: format || 'png'
        });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async TakeElementScreenshot(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, selector, format, quality } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const element = await session.page.$(selector);
        if (!element) {
          callback(null, { success: false, error: 'Element not found' });
          return;
        }

        const imageData = await element.screenshot({
          type: format === 'jpeg' ? 'jpeg' : 'png',
          quality: format === 'jpeg' ? (quality || 80) : undefined
        });

        callback(null, { success: true, image_data: imageData });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    // Network interception
    async StartNetworkCapture(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, url_patterns, capture_request_body, capture_response_body } = call.request;
        const success = await sessionManager.startNetworkCapture(
          session_id,
          url_patterns || [],
          capture_request_body || false,
          capture_response_body || false
        );

        callback(null, { success });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async StopNetworkCapture(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const success = sessionManager.stopNetworkCapture(call.request.session_id);
        callback(null, { success });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async GetCapturedRequests(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, clear_after } = call.request;
        const requests = sessionManager.getCapturedRequests(session_id, clear_after || false);

        callback(null, {
          success: true,
          requests: requests.map(r => ({
            request: {
              request_id: r.requestId,
              method: r.method,
              url: r.url,
              headers: r.headers,
              body: r.body,
              resource_type: r.resourceType,
              timestamp: r.timestamp
            },
            response: r.response ? {
              request_id: r.requestId,
              status_code: r.response.statusCode,
              status_text: r.response.statusText,
              headers: r.response.headers,
              body: r.response.body,
              timestamp: r.response.timestamp
            } : undefined
          }))
        });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async SetRequestInterceptor(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, url_pattern, action } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        await session.page.route(url_pattern, async (route) => {
          switch (action) {
            case 'INTERCEPT_ABORT':
              await route.abort();
              break;
            case 'INTERCEPT_FULFILL':
              await route.fulfill({ status: 200 });
              break;
            default:
              await route.continue();
          }
        });

        callback(null, { success: true });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    // Crawler support
    async GetPageLinks(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id, same_origin_only, include_external } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const currentUrl = new URL(session.page.url());
        const links = await session.page.evaluate(() => {
          return Array.from(document.querySelectorAll('a[href]')).map(a => ({
            href: (a as HTMLAnchorElement).href,
            text: a.textContent?.trim() || '',
            rel: a.getAttribute('rel') || ''
          }));
        });

        const processed = links.map(link => {
          try {
            const linkUrl = new URL(link.href);
            const isExternal = linkUrl.origin !== currentUrl.origin;
            return { ...link, is_external: isExternal };
          } catch {
            return { ...link, is_external: true };
          }
        });

        let filtered = processed;
        if (same_origin_only) {
          filtered = filtered.filter(l => !l.is_external);
        }
        if (!include_external) {
          filtered = filtered.filter(l => !l.is_external);
        }

        callback(null, { success: true, links: filtered });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    },

    async GetPageForms(
      call: { request: any },
      callback: GrpcCallback<any>
    ) {
      try {
        const { session_id } = call.request;
        const session = sessionManager.getSession(session_id);
        if (!session) {
          callback(null, { success: false, error: 'Session not found' });
          return;
        }

        const forms = await session.page.evaluate(() => {
          return Array.from(document.querySelectorAll('form')).map(form => {
            const fields = Array.from(form.querySelectorAll('input, select, textarea')).map(field => {
              const el = field as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
              const label = document.querySelector(`label[for="${el.id}"]`)?.textContent?.trim() || '';

              let options: string[] = [];
              if (el.tagName === 'SELECT') {
                options = Array.from((el as HTMLSelectElement).options).map(o => o.value);
              }

              return {
                name: el.name || '',
                type: el.type || 'text',
                id: el.id || '',
                placeholder: (el as HTMLInputElement).placeholder || '',
                required: el.required || false,
                value: el.value || '',
                options,
                label
              };
            });

            const submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');
            let submitSelector = '';
            if (submitBtn) {
              if (submitBtn.id) {
                submitSelector = `#${submitBtn.id}`;
              } else if (submitBtn.className) {
                submitSelector = `.${submitBtn.className.split(' ')[0]}`;
              } else {
                submitSelector = 'button[type="submit"]';
              }
            }

            return {
              action: form.action || '',
              method: form.method || 'get',
              id: form.id || '',
              name: form.name || '',
              fields,
              submit_button_selector: submitSelector
            };
          });
        });

        callback(null, { success: true, forms });
      } catch (err: any) {
        callback(null, { success: false, error: err.message });
      }
    }
  };
}
