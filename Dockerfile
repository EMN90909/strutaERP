FROM node:18-alpine

RUN apk add --no-cache unzip

WORKDIR /app

COPY struta-erp.zip .

RUN unzip struta-erp.zip && rm struta-erp.zip

# Find where package.json is and move there
RUN find /app -name package.json

# 👇 manually adjust this after you see output
WORKDIR /app/struta-erp

RUN npm install

EXPOSE 3000 5173

CMD sh -c "npx convex dev --hostname 0.0.0.0 & npm run dev"
