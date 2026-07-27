FROM node:22-slim

WORKDIR /app

COPY package*.json ./

RUN npm ci --omit=dev \
 && npm cache clean --force

COPY . .

ENV PORT 80

EXPOSE 80

CMD ["node", "server.js"]
