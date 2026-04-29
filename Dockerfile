# Use Node base image
FROM node:18-alpine

# Install unzip
RUN apk add --no-cache unzip

# Set working directory
WORKDIR /app

# Copy the zip file into container
COPY struta-erp.zip .

# Extract it
RUN unzip struta-erp.zip && rm struta-erp.zip

# Move into extracted folder (adjust if needed)
WORKDIR /app/struta-erp

# Install dependencies
RUN npm install

# Copy env (you should override in docker run or use docker-compose)
COPY .env .env

# Expose ports
EXPOSE 3000 5173

# Start both backend + frontend
CMD sh -c "npx convex dev & npm run dev"
