// MongoDB initialization script for Rocketship testing
// This script sets up test databases and collections for MongoDB plugin testing

// Switch to testdb database
db = db.getSiblingDB('testdb');

// Create a test user with read/write permissions
db.createUser({
  user: 'testuser',
  pwd: 'testpass',
  roles: [
    {
      role: 'readWrite',
      db: 'testdb'
    }
  ]
});

// Create test collections with sample data
db.createCollection('test_collection');

// Insert sample documents for testing
db.test_collection.insertMany([
  {
    name: 'Test Document 1',
    value: 100,
    active: true,
    tags: ['test', 'sample'],
    created_at: new Date()
  },
  {
    name: 'Test Document 2',
    value: 200,
    active: false,
    tags: ['test', 'example'],
    created_at: new Date()
  }
]);

// Create indexes for testing
db.test_collection.createIndex({ name: 1 }, { unique: false });
db.test_collection.createIndex({ value: 1 }, { background: true });

// Create additional test databases for different test scenarios
db = db.getSiblingDB('rocketship_test');

// Create user for rocketship_test database
db.createUser({
  user: 'testuser',
  pwd: 'testpass',
  roles: [
    {
      role: 'readWrite',
      db: 'rocketship_test'
    }
  ]
});

// Create collections for comprehensive testing
db.createCollection('users');
db.createCollection('products');
db.createCollection('orders');

// Insert sample data for users collection
db.users.insertMany([
  {
    name: 'John Doe',
    email: 'john@example.com',
    age: 30,
    active: true,
    role: 'developer',
    tags: ['javascript', 'mongodb'],
    created_at: new Date()
  },
  {
    name: 'Jane Smith',
    email: 'jane@example.com',
    age: 28,
    active: true,
    role: 'designer',
    tags: ['ui', 'ux'],
    created_at: new Date()
  },
  {
    name: 'Bob Wilson',
    email: 'bob@example.com',
    age: 35,
    active: false,
    role: 'manager',
    tags: ['management', 'strategy'],
    created_at: new Date()
  }
]);

// Insert sample data for products collection
db.products.insertMany([
  {
    name: 'Laptop',
    price: 999.99,
    category: 'Electronics',
    in_stock: true,
    quantity: 50,
    specs: {
      cpu: 'Intel i7',
      ram: '16GB',
      storage: '512GB SSD'
    }
  },
  {
    name: 'Mouse',
    price: 29.99,
    category: 'Accessories',
    in_stock: true,
    quantity: 100,
    specs: {
      type: 'Wireless',
      battery: 'AA',
      color: 'Black'
    }
  }
]);

// Create indexes for better query performance
db.users.createIndex({ email: 1 }, { unique: true });
db.users.createIndex({ age: 1 });
db.users.createIndex({ tags: 1 });
db.products.createIndex({ name: 1 });
db.products.createIndex({ category: 1, price: 1 });

print('MongoDB test databases initialized successfully!');
print('Available databases: testdb, rocketship_test');
print('Test user: testuser / testpass');
print('Sample collections and data have been created.');