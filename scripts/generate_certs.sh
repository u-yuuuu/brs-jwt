#!/bin/bash
# Генерация сертификатов для mTLS

mkdir -p certs
cd certs

# 1. Root CA (самоподписной)
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout ca-key.pem -out ca-cert.pem \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=BRS/CN=BRS-Root-CA"

# 2. Сертификат сервера
openssl req -newkey rsa:4096 -nodes \
  -keyout server-key.pem -out server-req.pem \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=BRS/CN=localhost"

openssl x509 -req -in server-req.pem -days 365 \
  -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
  -out server-cert.pem \
  -extfile <(echo "subjectAltName=DNS:localhost,IP:127.0.0.1")

# 3. Сертификат клиента (для сервиса)
openssl req -newkey rsa:4096 -nodes \
  -keyout client-key.pem -out client-req.pem \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=BRS/CN=brs-client"

openssl x509 -req -in client-req.pem -days 365 \
  -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
  -out client-cert.pem

echo "Certificates generated in ./certs/"