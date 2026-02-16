# easyweb3-cli (v2)

Go CLI used by OpenClaw via `exec` to call the easyweb3 PaaS API.

## Build

```bash
cd v2/easyweb3-cli

go build -o ./bin/easyweb3 .
```

## Usage

```bash
./bin/easyweb3 --api-base http://localhost:8080 auth login --api-key ew3_admin_dev
./bin/easyweb3 --api-base http://localhost:8080 auth status
./bin/easyweb3 --api-base http://localhost:8080 log create --action trade_executed --details '{"foo":"bar"}'
./bin/easyweb3 --api-base http://localhost:8080 log list --limit 20
./bin/easyweb3 --api-base http://localhost:8080 notify broadcast --message "trade ok" --event trade_executed
./bin/easyweb3 --api-base http://localhost:8080 integrations query --provider dexscreener --method search --params '{"q":"pepe"}'
./bin/easyweb3 --api-base http://localhost:8080 cache put --key foo --value '{"bar":1}' --ttl-seconds 60
./bin/easyweb3 --api-base http://localhost:8080 cache get foo
./bin/easyweb3 --api-base http://localhost:8080 service list
./bin/easyweb3 --api-base http://localhost:8080 service docs --name polymarket
./bin/easyweb3 --api-base http://localhost:8080 api raw --service polymarket --method GET --path /healthz
./bin/easyweb3 --api-base http://localhost:8080 api polymarket catalog-events --limit 1
./bin/easyweb3 --api-base http://localhost:8080 api polymarket executions --limit 5
```

Credentials are persisted to `~/.easyweb3/credentials.json`.
