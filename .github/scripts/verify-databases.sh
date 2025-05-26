#!/bin/bash
set -e

echo "Verifying database setup..."

# Verify PostgreSQL setup
PG_COUNT=$(PGPASSWORD=testpass psql -h localhost -p 5433 -U testuser -d testdb -t -c "SELECT COUNT(*) FROM users;" | xargs)
if [ "$PG_COUNT" -eq 4 ]; then
  echo "✅ PostgreSQL setup verified: $PG_COUNT users"
else
  echo "❌ PostgreSQL setup failed: expected 4 users, got $PG_COUNT"
  exit 1
fi

# Verify MySQL setup  
MYSQL_COUNT=$(mysql -h 127.0.0.1 -P 3306 -u root -ptestpass testdb -sN -e "SELECT COUNT(*) FROM users;")
if [ "$MYSQL_COUNT" -eq 4 ]; then
  echo "✅ MySQL setup verified: $MYSQL_COUNT users"
else
  echo "❌ MySQL setup failed: expected 4 users, got $MYSQL_COUNT"
  exit 1
fi

echo "✅ Both databases verified successfully!"