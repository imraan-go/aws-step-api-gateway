FROM alpine:latest as certs
RUN apk --update add ca-certificates
FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY * ./
COPY config.env ./
COPY *.crt ./
COPY *.key ./
EXPOSE 2000/tcp
EXPOSE 2100/tcp
ENTRYPOINT ["/aws-step-order-service"]