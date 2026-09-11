FROM node:26.8-alpine3.24 AS deps
WORKDIR /app
COPY package.json ./
RUN npm install --no-audit --no-fund

FROM node:26.8-alpine3.24 AS build
WORKDIR /app
ARG TELEMETRYFORGE_API_BASE=http://gateway:8080
ENV TELEMETRYFORGE_API_BASE=${TELEMETRYFORGE_API_BASE}
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:26.8-alpine3.24 AS runtime
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
USER node
EXPOSE 3000
CMD ["node", "server.js"]
