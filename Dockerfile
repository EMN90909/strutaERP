FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go build -o /out/struta-erp .

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/struta-erp /app/struta-erp
COPY templates /app/templates
COPY static /app/static

ENV PORT=8080
EXPOSE 8080

CMD ["/app/struta-erp"]
