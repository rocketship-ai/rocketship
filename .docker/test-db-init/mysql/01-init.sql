-- MySQL initialization script for Rocketship SQL plugin testing

USE testdb;

-- Create users table
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    age INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Create products table
CREATE TABLE products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    category VARCHAR(100),
    in_stock BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create orders table
CREATE TABLE orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    product_id INT,
    quantity INT NOT NULL DEFAULT 1,
    total_amount DECIMAL(10,2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
);

-- Insert sample users
INSERT INTO users (name, email, age) VALUES
    ('Alice Johnson', 'alice@example.com', 28),
    ('Bob Smith', 'bob@example.com', 34),
    ('Carol Davis', 'carol@example.com', 25),
    ('David Wilson', 'david@example.com', 31);

-- Insert sample products
INSERT INTO products (name, description, price, category) VALUES
    ('Laptop Pro', 'High-performance laptop for professionals', 1299.99, 'Electronics'),
    ('Wireless Mouse', 'Ergonomic wireless mouse with long battery life', 29.99, 'Electronics'),
    ('Coffee Mug', 'Ceramic coffee mug with company logo', 12.99, 'Office'),
    ('Notebook Set', 'Pack of 3 premium notebooks', 24.99, 'Office'),
    ('Smartphone', 'Latest generation smartphone with advanced features', 899.99, 'Electronics');

-- Insert sample orders
INSERT INTO orders (user_id, product_id, quantity, total_amount, status) VALUES
    (1, 1, 1, 1299.99, 'completed'),
    (1, 2, 2, 59.98, 'completed'),
    (2, 3, 1, 12.99, 'pending'),
    (3, 4, 3, 74.97, 'shipped'),
    (4, 5, 1, 899.99, 'pending');

-- Create indexes for performance testing
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);

-- Grant permissions to test user
GRANT ALL PRIVILEGES ON testdb.* TO 'testuser'@'%';