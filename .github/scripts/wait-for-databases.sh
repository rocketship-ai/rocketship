#!/bin/bash
set -e

echo "Waiting for databases to be ready..."

# Wait for PostgreSQL to be ready
echo "Waiting for PostgreSQL..."
for i in {1..30}; do
  if PGPASSWORD=testpass psql -h localhost -p 5433 -U testuser -d testdb -c "SELECT 1" > /dev/null 2>&1; then
    echo "✅ PostgreSQL is ready!"
    break
  fi
  echo "PostgreSQL not ready, waiting... ($i/30)"
  sleep 2
done

# Wait for MySQL to be ready
echo "Waiting for MySQL..."
for i in {1..30}; do
  if mysql -h 127.0.0.1 -P 3306 -u root -ptestpass testdb -e "SELECT 1" > /dev/null 2>&1; then
    echo "✅ MySQL is ready!"
    break
  fi
  echo "MySQL not ready, waiting... ($i/30)"
  sleep 2
done

echo "✅ Both databases are ready!"