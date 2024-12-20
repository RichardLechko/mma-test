FROM node:20 AS build

WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm install

# Copy the project files
COPY . .

# Build the Astro app
RUN npm run build

FROM node:20
WORKDIR /app
COPY --from=build /app/package*.json ./
COPY --from=build /app/dist/ ./dist/
COPY --from=build /app/node_modules/ ./node_modules/

EXPOSE 4321
# Use the host flag to make it accessible outside the container
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0"]