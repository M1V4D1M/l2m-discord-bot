#!/bin/bash

# Test PING
echo "Testing PING..."
curl -X POST http://localhost:8080/interactions \
  -H "Content-Type: application/json" \
  -d '{"type": 1}'
echo -e "\n"

# Test /scrolls
echo "Testing /scrolls..."
curl -X POST http://localhost:8080/interactions \
  -H "Content-Type: application/json" \
  -d '{
    "type": 2,
    "token": "test_token_scrolls",
    "id": "interaction_id_1",
    "channel_id": "1333093445559910520",
    "guild_id": "1113785029399687190",
    "data": {
      "name": "scrolls"
    },
    "member": {
      "user": {
        "id": "12345",
        "username": "tester"
      }
    }
  }'
echo -e "\n"

# Test /roll
echo "Testing /roll..."
curl -X POST http://localhost:8080/interactions \
  -H "Content-Type: application/json" \
  -d '{
    "type": 2,
    "token": "test_token_roll",
    "id": "interaction_id_2",
    "channel_id": "1333093445559910520",
    "guild_id": "1113785029399687190",
    "data": {
      "name": "roll"
    },
    "member": {
      "user": {
        "id": "12345",
        "username": "tester"
      }
    }
  }'
echo -e "\n"
