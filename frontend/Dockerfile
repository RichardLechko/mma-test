FROM node:20 AS build

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

RUN npm run build

FROM node:20
WORKDIR /app
COPY --from=build /app/package*.json ./
COPY --from=build /app/dist/ ./dist/
COPY --from=build /app/node_modules/ ./node_modules/

EXPOSE 4321
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0"]