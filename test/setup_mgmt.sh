#!/bin/bash

set -e

# Install sqlite3
apt-get update && apt-get install -y sqlite3 curl jq

while true
do
    # Check if the schema is ready
    if sqlite3 /var/lib/netbird/store.db "SELECT name FROM sqlite_master WHERE type='table' AND name='accounts';" | grep -q accounts; then
        echo "Database schema detected."

        # Check if the database is empty
        if [ $(sqlite3 /var/lib/netbird/store.db "SELECT COUNT(*) FROM accounts;") -eq 0 ]; then
            echo "Seeding the database..."
            sqlite3 /var/lib/netbird/store.db < /app/seed_database.sql
            echo "Database seeded successfully."

            # Create proxy access token
            echo "Creating proxy access token..."
            TOKEN=$(/go/bin/netbird-mgmt token create \
                --config /etc/netbird/management.json \
                --name test-proxy \
                --log-file console 2>/dev/null \
                | grep "^Token:" | awk '{print $2}')

            if [ -n "$TOKEN" ]; then
                echo "NB_PROXY_TOKEN=$TOKEN" > /var/lib/netbird/proxy.env
                echo "Proxy token created successfully."
            else
                echo "ERROR: Failed to create proxy token"
                exit 1
            fi

            exit 0
        else
            echo "Database already contains data, skipping seeding."
            exit 1
        fi
    else
        echo "Database schema not found. Waiting..."
    fi
done
