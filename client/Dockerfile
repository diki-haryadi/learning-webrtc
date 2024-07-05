# Use the official Node.js 16.20.1 Alpine image as the base
FROM node:16.20.1-alpine

# Set the working directory to /app
WORKDIR /app

# Copy the rest of the application code
COPY . .

# Install the dependencies
# RUN npm install

# Build the React application
# RUN npm run build
RUN npm install -g serve

# Serve the built application
CMD ["serve", "-s", "build"]
