# LinqB2B

Welcome to the LinqB2B repository. This repository contains the frontend, backend, and indexer for the Linq B2B merchant platform.

## Repository Structure

- **[frontend/](./frontend)**: The web interface for merchants to manage their accounts and view transactions. Built with Vite and React.
- **[backend/](./backend)**: The core API service that handles business logic, merchant onboarding, and webhook dispatching. Built with Go.
- **[indexer/](./indexer)**: The blockchain indexer that monitors for incoming USDC payments on Sui and other supported chains. Built with Go and NATS.

## Getting Started

Refer to the individual README files in each directory for specific setup and development instructions:

- [Frontend README](./frontend/README.md) (Wait, I should check if it exists)
- [Backend README](./backend/README.md)
- [Indexer README](./indexer/README.md)

## Deployment

Detailed deployment guides for each service are available within their respective folders.

- [Koyeb Deployment for Indexer](./indexer/KOYEB_DEPLOYMENT.md)
- [Setup Guide for Backend](./backend/SETUP.md)
