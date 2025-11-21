const express = require('express');
const app = express();

app.use(express.json());

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});

// User routes
app.get('/users', getAllUsers);
app.get('/users/:id', getUserById);
app.post('/users', createUser);
app.put('/users/:id', updateUser);
app.delete('/users/:id', deleteUser);

// Product routes
app.get('/products', getProducts);
app.get('/products/:productId', getProductById);
app.post('/products', authenticate, createProduct);

// Order routes with middleware
app.get('/orders', authenticate, getOrders);
app.post('/orders', authenticate, validateOrder, createOrder);

// Helper functions
function getAllUsers(req, res) {
  res.json([{ id: 1, name: 'John' }]);
}

function getUserById(req, res) {
  const { id } = req.params;
  res.json({ id, name: 'John' });
}

function createUser(req, res) {
  const user = req.body;
  res.status(201).json({ id: 1, ...user });
}

function updateUser(req, res) {
  const { id } = req.params;
  res.json({ id, ...req.body });
}

function deleteUser(req, res) {
  res.status(204).send();
}

function getProducts(req, res) {
  res.json([{ id: 1, name: 'Widget', price: 9.99 }]);
}

function getProductById(req, res) {
  const { productId } = req.params;
  res.json({ id: productId, name: 'Widget', price: 9.99 });
}

function createProduct(req, res) {
  res.status(201).json({ id: 1, ...req.body });
}

function getOrders(req, res) {
  res.json([{ id: 1, total: 99.99 }]);
}

function createOrder(req, res) {
  res.status(201).json({ id: 1, ...req.body });
}

// Middleware
function authenticate(req, res, next) {
  const token = req.headers.authorization;
  if (!token) {
    return res.status(401).json({ error: 'Unauthorized' });
  }
  next();
}

function validateOrder(req, res, next) {
  if (!req.body.items || req.body.items.length === 0) {
    return res.status(400).json({ error: 'Order must have items' });
  }
  next();
}

module.exports = app;
