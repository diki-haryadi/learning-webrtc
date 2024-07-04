# Use the official Node.js 16.20.1 Alpine image as the base
FROM node:16.20.1-alpine

# Set the working directory to /app
WORKDIR /app

# Copy the package.json and package-lock.json files
COPY package*.json ./

# Install the dependencies
RUN npm ci

# Copy the rest of the application code
COPY . .

# Build the React application
RUN npm run build

# Serve the built application
CMD ["npm", "run", "serve", "-s", "build"]
