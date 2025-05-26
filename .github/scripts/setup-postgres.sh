#!/bin/bash
set -e

echo "Setting up PostgreSQL schema..."

PGPASSWORD=testpass psql -h localhost -p 5433 -U testuser -d testdb << 'EOF'
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    age INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Products table
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100),
    price DECIMAL(10, 2) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    product_id INTEGER REFERENCES products(id),
    quantity INTEGER NOT NULL DEFAULT 1,
    total_amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO users (name, email, age) VALUES 
    ('Alice Johnson', 'alice@example.com', 28),
    ('Bob Smith', 'bob@example.com', 34),
    ('Carol Davis', 'carol@example.com', 22),
    ('David Wilson', 'david@example.com', 41);

INSERT INTO products (name, category, price, description) VALUES 
    ('Laptop Pro', 'Electronics', 1299.99, 'High-performance laptop'),
    ('Smartphone X', 'Electronics', 899.99, 'Latest smartphone model'),
    ('Office Chair', 'Furniture', 299.99, 'Ergonomic office chair'),
    ('Desk Lamp', 'Furniture', 79.99, 'LED desk lamp'),
    ('Wireless Mouse', 'Electronics', 49.99, 'Bluetooth wireless mouse');

INSERT INTO orders (user_id, product_id, quantity, total_amount, status) VALUES 
    (1, 1, 1, 1299.99, 'completed'),
    (1, 5, 2, 99.98, 'completed'),
    (2, 2, 1, 899.99, 'pending'),
    (3, 3, 1, 299.99, 'processing'),
    (4, 4, 1, 79.99, 'completed');
EOF

echo "✅ PostgreSQL schema setup completed"