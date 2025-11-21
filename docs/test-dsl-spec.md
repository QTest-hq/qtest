# Test DSL Specification v1.0

## 1. Overview

The Test DSL is an intermediate representation for test definitions. It provides a framework-agnostic way to describe tests, which are then converted to specific test framework code (Jest, Pytest, JUnit, Playwright) by adapters.

### 1.1 Design Goals

1. **Framework-agnostic**: Same DSL works for all test types and frameworks
2. **Complete**: Captures all information needed to generate runnable tests
3. **Validatable**: Schema can be validated before generation
4. **Human-readable**: YAML format for easy inspection and debugging
5. **Extensible**: Support custom fields for framework-specific features

### 1.2 Format

The DSL uses YAML as its serialization format. JSON is also supported for programmatic use.

## 2. Schema Definition

### 2.1 Top-Level Structure

```yaml
# Required: Test identification
test:
  id: string              # Unique identifier (e.g., "unit.user-service.create-user.happy")
  type: TestType          # "unit" | "integration" | "api" | "e2e"
  level: TestLevel        # "unit" | "integration" | "e2e"
  description: string     # Human-readable description

# Required: What is being tested
target:
  kind: TargetKind        # "function" | "method" | "endpoint" | "flow"
  # Additional fields based on kind (see below)

# Optional: Test setup
setup:
  - SetupStep             # Array of setup operations

# Optional: Test teardown
teardown:
  - TeardownStep          # Array of teardown operations

# Required for unit/integration/api: Test inputs
input:
  # Varies by target kind (see below)

# Required: Expected outcomes
expect:
  # Varies by target kind (see below)

# Optional: Lifecycle configuration
lifecycle:
  scope: Scope            # "test" | "suite" | "global"
  setup: string[]         # Setup commands
  teardown: string[]      # Teardown commands

# Optional: Resource requirements
resources:
  # Resource definitions (see below)

# Optional: Isolation configuration
isolation:
  level: IsolationLevel   # "none" | "db-transaction" | "process"
  parallelSafe: boolean   # Whether test can run in parallel
```

### 2.2 Enums

```typescript
type TestType = "unit" | "integration" | "api" | "e2e";

type TestLevel = "unit" | "integration" | "e2e";

type TargetKind = "function" | "method" | "endpoint" | "flow";

type Scope = "test" | "suite" | "global";

type IsolationLevel = "none" | "db-transaction" | "process";

type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "HEAD" | "OPTIONS";

type AssertionType = "equals" | "contains" | "matches" | "throws" | "resolves" | "rejects";
```

## 3. Target Specifications

### 3.1 Function Target

For unit testing standalone functions.

```yaml
target:
  kind: "function"
  module: string          # Module path (e.g., "src/utils/math")
  name: string            # Function name (e.g., "calculateTotal")
  async: boolean          # Whether function is async
```

**Example:**
```yaml
target:
  kind: "function"
  module: "src/utils/pricing"
  name: "calculateDiscount"
  async: false
```

### 3.2 Method Target

For testing class methods.

```yaml
target:
  kind: "method"
  module: string          # Module path
  class: string           # Class name
  method: string          # Method name
  static: boolean         # Whether method is static
  async: boolean          # Whether method is async
```

**Example:**
```yaml
target:
  kind: "method"
  module: "src/services/UserService"
  class: "UserService"
  method: "createUser"
  static: false
  async: true
```

### 3.3 Endpoint Target

For API testing HTTP endpoints.

```yaml
target:
  kind: "endpoint"
  method: HttpMethod      # HTTP method
  path: string            # Route path (e.g., "/users/:id")
  handler: string         # Optional: Handler reference
  baseUrl: string         # Optional: Base URL for requests
```

**Example:**
```yaml
target:
  kind: "endpoint"
  method: "POST"
  path: "/api/users"
  handler: "UserController.create"
  baseUrl: "http://localhost:3000"
```

### 3.4 Flow Target

For E2E testing user flows.

```yaml
target:
  kind: "flow"
  name: string            # Flow name (e.g., "user-login")
  startUrl: string        # Starting URL
  description: string     # Flow description
```

**Example:**
```yaml
target:
  kind: "flow"
  name: "checkout"
  startUrl: "/cart"
  description: "Complete checkout process"
```

## 4. Input Specifications

### 4.1 Function/Method Input

```yaml
input:
  args:                   # Positional arguments
    - value: any          # Argument value
      type: string        # Optional: Type hint
  kwargs:                 # Named arguments (Python)
    key: value
  thisArg: any            # Optional: 'this' context for methods
```

**Example:**
```yaml
input:
  args:
    - value: [{ id: 1, price: 100 }, { id: 2, price: 50 }]
      type: "CartItem[]"
    - value: 0.1
      type: "number"
```

### 4.2 Endpoint Input

```yaml
input:
  params:                 # URL parameters
    key: value
  query:                  # Query string parameters
    key: value
  headers:                # Request headers
    key: value
  body:                   # Request body
    key: value
  cookies:                # Cookies
    key: value
  auth:                   # Authentication
    type: "bearer" | "basic" | "api-key"
    value: string
```

**Example:**
```yaml
input:
  params:
    id: "123"
  headers:
    Content-Type: "application/json"
  body:
    name: "John Doe"
    email: "john@example.com"
  auth:
    type: "bearer"
    value: "{{TEST_TOKEN}}"
```

### 4.3 Flow Input (E2E)

For E2E tests, input is embedded in steps rather than declared separately.

## 5. Expectation Specifications

### 5.1 Function/Method Expectations

```yaml
expect:
  returns:                # Expected return value
    value: any            # Exact value or matcher
    type: string          # Optional: Type check
  throws:                 # Expected error
    type: string          # Error class name
    message: string       # Error message (exact or regex)
  calls:                  # Spy/mock assertions
    - target: string      # What was called
      times: number       # How many times
      with: any[]         # With what arguments
  sideEffects:            # Side effect assertions
    - type: string        # Effect type
      assertion: any      # Effect-specific assertion
```

**Example:**
```yaml
expect:
  returns:
    value: 135
    type: "number"
  calls:
    - target: "logger.info"
      times: 1
      with: ["Discount applied: 10%"]
```

### 5.2 Endpoint Expectations

```yaml
expect:
  status: number          # HTTP status code
  headers:                # Response headers
    key: value
  body:                   # Response body assertions
    key: value | Matcher
  timing:                 # Performance assertions
    maxMs: number         # Maximum response time
  db:                     # Database side effects
    check:
      table: string
      where: object
      exists: boolean
    count:
      table: string
      where: object
      value: number
```

**Example:**
```yaml
expect:
  status: 201
  headers:
    Content-Type: "application/json"
  body:
    id: "{{any.uuid}}"
    name: "John Doe"
    email: "john@example.com"
    createdAt: "{{any.iso8601}}"
  db:
    check:
      table: "users"
      where: { email: "john@example.com" }
      exists: true
```

### 5.3 Flow Expectations (E2E)

E2E expectations are embedded in steps (see section 6).

## 6. Step Definitions (E2E)

E2E tests use a step-based format for describing user interactions.

### 6.1 Step Types

```yaml
steps:
  # Navigation
  - goto: string                    # Navigate to URL

  # Interactions
  - click: Selector                 # Click element
  - fill: { selector: S, value: V } # Fill input
  - select: { selector: S, value: V } # Select dropdown
  - check: Selector                 # Check checkbox
  - uncheck: Selector               # Uncheck checkbox
  - hover: Selector                 # Hover over element
  - press: string                   # Press key
  - upload: { selector: S, file: F } # Upload file

  # Waiting
  - wait: number                    # Wait milliseconds
  - waitFor: Selector               # Wait for element
  - waitForNavigation: boolean      # Wait for navigation
  - waitForNetwork: string          # Wait for network request

  # Assertions
  - expect: Assertion               # Assert condition

  # Screenshots
  - screenshot: string              # Take screenshot
```

### 6.2 Selector Format

```yaml
selector:
  css: string             # CSS selector
  # OR
  xpath: string           # XPath selector
  # OR
  text: string            # Text content match
  # OR
  testId: string          # data-testid attribute
  # OR
  role: string            # ARIA role
  name: string            # Accessible name
```

### 6.3 E2E Assertions

```yaml
expect:
  selector: Selector      # Element to assert on
  visible: boolean        # Element is visible
  hidden: boolean         # Element is hidden
  text: string            # Element text content
  value: string           # Input value
  attribute:
    name: string
    value: string
  url: string             # Current URL matches
  title: string           # Page title matches
```

### 6.4 Complete E2E Example

```yaml
test:
  id: "e2e.auth.login.success"
  type: "e2e"
  level: "e2e"
  description: "User can log in with valid credentials"

target:
  kind: "flow"
  name: "login"
  startUrl: "/login"
  description: "User authentication flow"

steps:
  - goto: "/login"

  - fill:
      selector: { testId: "email-input" }
      value: "test@example.com"

  - fill:
      selector: { testId: "password-input" }
      value: "password123"

  - click:
      selector: { testId: "submit-button" }

  - waitForNavigation: true

  - expect:
      url: "/dashboard"

  - expect:
      selector: { testId: "welcome-message" }
      visible: true
      text: "Welcome, Test User"

lifecycle:
  scope: "test"
  setup:
    - "db:seed users"
  teardown:
    - "db:clean users"

isolation:
  level: "db-transaction"
  parallelSafe: false
```

## 7. Setup and Teardown

### 7.1 Setup Steps

```yaml
setup:
  - db: string            # Database operation
  - seed: string          # Seed data
  - mock: MockDef         # Set up mock
  - env: { key: value }   # Set environment variable
  - exec: string          # Execute command
  - fixture: string       # Load fixture file
```

**Example:**
```yaml
setup:
  - db: "clean users"
  - seed: "users with admin"
  - mock:
      target: "EmailService.send"
      returns: { success: true }
  - env:
      NODE_ENV: "test"
```

### 7.2 Teardown Steps

```yaml
teardown:
  - db: string            # Database cleanup
  - restore: string       # Restore mock
  - exec: string          # Execute command
```

## 8. Resource Definitions

### 8.1 Database Resource

```yaml
resources:
  db:
    type: "postgres" | "mysql" | "sqlite" | "mongodb"
    mode: "testcontainer" | "inmemory" | "shared"
    migrations: boolean   # Run migrations
    seed: string          # Seed file path
```

### 8.2 Cache Resource

```yaml
resources:
  cache:
    type: "redis" | "memcached"
    mode: "testcontainer" | "shared"
```

### 8.3 External Service Resource

```yaml
resources:
  services:
    - name: string
      type: "mock" | "testcontainer" | "real"
      image: string       # For testcontainer
      port: number
```

## 9. Matchers

Matchers allow flexible assertions beyond exact equality.

### 9.1 Built-in Matchers

```yaml
# Any value of type
"{{any.string}}"
"{{any.number}}"
"{{any.boolean}}"
"{{any.array}}"
"{{any.object}}"
"{{any.uuid}}"
"{{any.email}}"
"{{any.url}}"
"{{any.iso8601}}"

# Comparison matchers
"{{gt:10}}"              # Greater than
"{{gte:10}}"             # Greater than or equal
"{{lt:10}}"              # Less than
"{{lte:10}}"             # Less than or equal
"{{between:1,10}}"       # Between (inclusive)

# String matchers
"{{startsWith:Hello}}"
"{{endsWith:World}}"
"{{contains:test}}"
"{{matches:^[a-z]+$}}"   # Regex match
"{{length:10}}"          # Exact length
"{{minLength:5}}"
"{{maxLength:100}}"

# Array matchers
"{{arrayContaining:[1,2,3]}}"
"{{arrayLength:5}}"

# Object matchers
"{{objectContaining:{key: value}}}"
"{{hasKey:fieldName}}"
```

### 9.2 Matcher Examples

```yaml
expect:
  body:
    id: "{{any.uuid}}"
    email: "{{any.email}}"
    createdAt: "{{any.iso8601}}"
    age: "{{gte:18}}"
    name: "{{minLength:2}}"
    tags: "{{arrayContaining:['active']}}"
```

## 10. Variables and Templating

### 10.1 Environment Variables

```yaml
input:
  auth:
    value: "{{env.TEST_TOKEN}}"
```

### 10.2 Fixture References

```yaml
input:
  body: "{{fixture.validUser}}"
```

### 10.3 Generated Values

```yaml
input:
  body:
    email: "{{generate.email}}"
    id: "{{generate.uuid}}"
```

### 10.4 Response Capture

```yaml
steps:
  - capture:
      from: "response.body.id"
      as: "userId"

  - goto: "/users/{{captured.userId}}"
```

## 11. Complete Examples

### 11.1 Unit Test Example

```yaml
test:
  id: "unit.utils.pricing.calculate-discount.percentage"
  type: "unit"
  level: "unit"
  description: "Calculates percentage discount correctly"

target:
  kind: "function"
  module: "src/utils/pricing"
  name: "calculateDiscount"
  async: false

input:
  args:
    - value: 100
      type: "number"
    - value: 0.2
      type: "number"

expect:
  returns:
    value: 80
    type: "number"

isolation:
  level: "none"
  parallelSafe: true
```

### 11.2 Integration Test Example

```yaml
test:
  id: "integration.user-service.create.with-notifications"
  type: "integration"
  level: "integration"
  description: "Creating user sends welcome email"

target:
  kind: "method"
  module: "src/services/UserService"
  class: "UserService"
  method: "create"
  static: false
  async: true

setup:
  - db: "clean users"
  - mock:
      target: "EmailService.send"
      returns: { messageId: "123" }

input:
  args:
    - value:
        name: "John Doe"
        email: "john@example.com"

expect:
  returns:
    value:
      id: "{{any.uuid}}"
      name: "John Doe"
      email: "john@example.com"
  calls:
    - target: "EmailService.send"
      times: 1
      with:
        - to: "john@example.com"
          template: "welcome"

teardown:
  - db: "clean users"

resources:
  db:
    type: "postgres"
    mode: "testcontainer"
    migrations: true

isolation:
  level: "db-transaction"
  parallelSafe: true
```

### 11.3 API Test Example

```yaml
test:
  id: "api.users.create.success"
  type: "api"
  level: "integration"
  description: "POST /users creates new user"

target:
  kind: "endpoint"
  method: "POST"
  path: "/api/users"
  baseUrl: "http://localhost:3000"

setup:
  - db: "clean users"
  - seed: "users with admin"

input:
  headers:
    Content-Type: "application/json"
    Authorization: "Bearer {{env.ADMIN_TOKEN}}"
  body:
    name: "Jane Doe"
    email: "jane@example.com"
    role: "user"

expect:
  status: 201
  headers:
    Content-Type: "application/json"
  body:
    id: "{{any.uuid}}"
    name: "Jane Doe"
    email: "jane@example.com"
    role: "user"
    createdAt: "{{any.iso8601}}"
  db:
    check:
      table: "users"
      where: { email: "jane@example.com" }
      exists: true

teardown:
  - db: "rollback"

lifecycle:
  scope: "test"

resources:
  db:
    type: "postgres"
    mode: "testcontainer"

isolation:
  level: "db-transaction"
  parallelSafe: true
```

### 11.4 E2E Test Example

```yaml
test:
  id: "e2e.checkout.complete-purchase"
  type: "e2e"
  level: "e2e"
  description: "User completes checkout flow"

target:
  kind: "flow"
  name: "checkout"
  startUrl: "/products"
  description: "Product selection through payment"

steps:
  # Browse products
  - goto: "/products"
  - click:
      selector: { testId: "product-card-1" }
  - click:
      selector: { testId: "add-to-cart" }

  # Go to cart
  - click:
      selector: { testId: "cart-icon" }
  - expect:
      selector: { testId: "cart-item" }
      visible: true

  # Proceed to checkout
  - click:
      selector: { testId: "checkout-button" }
  - waitForNavigation: true
  - expect:
      url: "/checkout"

  # Fill shipping info
  - fill:
      selector: { testId: "shipping-name" }
      value: "Test User"
  - fill:
      selector: { testId: "shipping-address" }
      value: "123 Test St"
  - click:
      selector: { testId: "continue-button" }

  # Fill payment info
  - fill:
      selector: { testId: "card-number" }
      value: "4242424242424242"
  - fill:
      selector: { testId: "card-expiry" }
      value: "12/25"
  - fill:
      selector: { testId: "card-cvc" }
      value: "123"

  # Complete purchase
  - click:
      selector: { testId: "place-order" }
  - waitFor:
      selector: { testId: "order-confirmation" }
  - expect:
      selector: { testId: "order-confirmation" }
      visible: true
      text: "{{contains:Order confirmed}}"

  # Screenshot for records
  - screenshot: "checkout-complete"

lifecycle:
  scope: "test"
  setup:
    - "db:seed products"
    - "db:seed test-user"
  teardown:
    - "db:clean orders"

resources:
  db:
    type: "postgres"
    mode: "shared"

isolation:
  level: "process"
  parallelSafe: false
```

## 12. Validation Rules

### 12.1 Required Fields

| Test Type | Required Fields |
|-----------|-----------------|
| All | `test.id`, `test.type`, `test.description`, `target` |
| unit/integration | `input`, `expect` |
| api | `input`, `expect.status` |
| e2e | `steps` (at least one) |

### 12.2 Validation Errors

| Error Code | Description |
|------------|-------------|
| `DSL_001` | Missing required field |
| `DSL_002` | Invalid enum value |
| `DSL_003` | Invalid selector format |
| `DSL_004` | Invalid matcher syntax |
| `DSL_005` | Incompatible target/test type |
| `DSL_006` | Invalid resource configuration |
| `DSL_007` | Circular dependency in setup |

## 13. Adapter Requirements

Each framework adapter must support:

1. **Parse**: Read DSL YAML/JSON
2. **Validate**: Check DSL against schema
3. **Transform**: Convert DSL to framework code
4. **Generate**: Output runnable test file(s)

### 13.1 Jest Adapter Mapping

| DSL | Jest |
|-----|------|
| `test.description` | `describe`/`it` description |
| `setup` | `beforeEach`/`beforeAll` |
| `teardown` | `afterEach`/`afterAll` |
| `expect.returns` | `expect(result).toBe/toEqual` |
| `expect.throws` | `expect(() => fn()).toThrow` |

### 13.2 Playwright Adapter Mapping

| DSL | Playwright |
|-----|------------|
| `steps.goto` | `page.goto()` |
| `steps.click` | `page.click()` |
| `steps.fill` | `page.fill()` |
| `steps.expect` | `expect(locator).toBeVisible()` etc. |

## 14. Extensibility

### 14.1 Custom Fields

Adapters may define custom fields under a namespace:

```yaml
test:
  id: "custom.test"
  # ...

x-jest:
  timeout: 30000
  retry: 3

x-playwright:
  browserType: "chromium"
  viewport: { width: 1920, height: 1080 }
```

### 14.2 Custom Matchers

```yaml
expect:
  body:
    field: "{{custom.myMatcher:arg1,arg2}}"
```

Adapters register custom matcher implementations.

## 15. Versioning

The DSL version is specified in metadata:

```yaml
version: "1.0"
test:
  # ...
```

Adapters must validate version compatibility.
