FROM node:24-alpine
WORKDIR /app
COPY package.json server.mjs geo.mjs auth.mjs ./
COPY public ./public
RUN mkdir /app/data && chown node:node /app/data
USER node
ENV HOST=0.0.0.0 PORT=3080
EXPOSE 3080
VOLUME /app/data
CMD ["node","server.mjs"]
