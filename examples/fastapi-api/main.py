from fastapi import FastAPI, HTTPException, Depends
from pydantic import BaseModel
from typing import List, Optional

app = FastAPI(title="Sample API")

# Models
class User(BaseModel):
    id: Optional[int] = None
    name: str
    email: str

class Product(BaseModel):
    id: Optional[int] = None
    name: str
    price: float

class Order(BaseModel):
    id: Optional[int] = None
    user_id: int
    items: List[int]
    total: float

# In-memory storage
users_db = []
products_db = []
orders_db = []

# Health check
@app.get("/health")
def health_check():
    return {"status": "ok"}

# User endpoints
@app.get("/users")
def list_users():
    return users_db

@app.get("/users/{user_id}")
def get_user(user_id: int):
    for user in users_db:
        if user["id"] == user_id:
            return user
    raise HTTPException(status_code=404, detail="User not found")

@app.post("/users")
def create_user(user: User):
    user_dict = user.dict()
    user_dict["id"] = len(users_db) + 1
    users_db.append(user_dict)
    return user_dict

@app.put("/users/{user_id}")
def update_user(user_id: int, user: User):
    for i, existing in enumerate(users_db):
        if existing["id"] == user_id:
            users_db[i] = {**user.dict(), "id": user_id}
            return users_db[i]
    raise HTTPException(status_code=404, detail="User not found")

@app.delete("/users/{user_id}")
def delete_user(user_id: int):
    for i, user in enumerate(users_db):
        if user["id"] == user_id:
            del users_db[i]
            return {"deleted": True}
    raise HTTPException(status_code=404, detail="User not found")

# Product endpoints
@app.get("/products")
def list_products():
    return products_db

@app.get("/products/{product_id}")
def get_product(product_id: int):
    for product in products_db:
        if product["id"] == product_id:
            return product
    raise HTTPException(status_code=404, detail="Product not found")

@app.post("/products")
def create_product(product: Product):
    product_dict = product.dict()
    product_dict["id"] = len(products_db) + 1
    products_db.append(product_dict)
    return product_dict

# Order endpoints
@app.get("/orders")
async def list_orders():
    return orders_db

@app.post("/orders")
async def create_order(order: Order):
    order_dict = order.dict()
    order_dict["id"] = len(orders_db) + 1
    orders_db.append(order_dict)
    return order_dict

@app.get("/orders/{order_id}")
async def get_order(order_id: int):
    for order in orders_db:
        if order["id"] == order_id:
            return order
    raise HTTPException(status_code=404, detail="Order not found")
