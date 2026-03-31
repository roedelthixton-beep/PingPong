# Admin Console Frontend

React admin frontend built with Vite + Refine + Ant Design.

## Development

```bash
# Install dependencies
pnpm install

# Start dev server
pnpm dev

# Build for production
pnpm build

# Preview production build
pnpm preview
```

## Features

- **Agents Management**: View and filter agents by status and keywords
- **Items Management**: View and filter items by status and keywords
- **Impr Records**: Query agent impression records by `agent_id` and view matched item rows
- Pagination support
- Real-time filtering

## Configuration

Frontend API URL can be configured via environment variables:

```bash
# Configure in the repository root .env file
# Set your console API address
CONSOLE_API_URL=http://localhost:8090/console/api/v1
# Or just change the port
CONSOLE_API_PORT=8090
# Dev server port (Vite)
CONSOLE_WEBAPP_PORT=5173
```

`console/webapp` explicitly sets `envDir=../..` in [vite.config.ts](console/webapp/vite.config.ts), so it reads the repository root `.env` instead of `console/webapp/.env`.

If `CONSOLE_API_URL` is not configured, the frontend defaults to the current page host with `:${CONSOLE_API_PORT:-8090}/console/api/v1` (e.g., when accessing `http://localhost:5173`, it will request `http://localhost:8090/console/api/v1`).

## Tech Stack

- React 19
- TypeScript
- Vite 7
- Refine 5
- Ant Design 6
- React Router 7
