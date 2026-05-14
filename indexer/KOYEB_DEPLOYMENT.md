# 🚢 Koyeb Deployment Guide

This guide explains how to deploy the Multi-chain Indexer to Koyeb using the "All-in-One" Docker setup (which includes NATS & Redis inside the container).

## 1. Create a New Service
1. Go to the [Koyeb Console](https://app.koyeb.com/).
2. Click **Create Service**.
3. Choose **GitHub** as the source.
4. Select your repository: `indexer`.
5. Specify the branch: `quant-testnet`.

## 2. Configure Build & Run
- **Build Settings**: Koyeb will automatically detect the `Dockerfile`.
- **Run Command**: Keep the default (it will use the `CMD` from the Dockerfile, which starts `supervisord`).

## 3. Set Environment Variables
Add the following variables in the **Environment Variables** section:

| Variable | Value | Description |
| :--- | :--- | :--- |
| `DATABASE_URL` | `postgres://3938...` | Your Prisma DB URL (provided). |
| `MASTER_MNEMONIC` | `your recovery phrase...` | 12 or 24 word mnemonic for wallet derivation. |
| `WEBHOOK_URL` | `https://api.yoursite.com/webhook` | Where the dispatcher sends notifications. |
| `WEBHOOK_SECRET` | `your-secret-key` | Used to sign webhooks (HMAC-SHA256). |
| `PORT` | `8080` | Koyeb uses this for health checks. |

> [!IMPORTANT]
> Make sure your `DATABASE_URL` includes `?sslmode=require` as provided.

## 4. Port Configuration
1. In the **Exposed Ports** section, ensure port **8080** is exposed.
2. Set the **Protocol** to `HTTP`.
3. Set the **Path** to `/health`.

## 5. Verification
Once deployed, you can verify it's working by visiting:
`https://<your-app-name>.koyeb.app/health`

You should see:
```json
{
  "status": "ok",
  "timestamp": "2026-04-20T...",
  "version": "1.0.0"
}
```

## 🛠️ Troubleshooting
- **Logs**: Check the "Runtime Logs" in Koyeb. You should see messages from `supervisord` starting Redis, NATS, and the Indexer.
- **Connection Refused**: If the Indexer fails to connect to Redis or NATS, ensure `supervisord` is running them on `localhost:6379` and `localhost:4222`.
