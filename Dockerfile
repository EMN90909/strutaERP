FROM node:18-alpine

RUN apk add --no-cache unzip

WORKDIR /app

COPY struta-erp.zip .

RUN unzip struta-erp.zip && rm struta-erp.zip

WORKDIR /app/struta-erp

RUN npm install

EXPOSE 3000 5173

CMD sh -c "npx convex dev & npm run dev"
