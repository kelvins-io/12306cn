FROM nginx:1.27-alpine
RUN apk add --no-cache ca-certificates tzdata \
  && rm -f /etc/nginx/conf.d/default.conf
WORKDIR /app
COPY bin/server /app/server
ENV TZ=Asia/Shanghai
EXPOSE 8080
CMD ["/app/server"]
