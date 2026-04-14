FROM alpine:3.21
RUN adduser -D -u 1000 appuser
USER appuser
WORKDIR /app
COPY facade ./facade
COPY .env.example .env
COPY ruleset.yml ./ruleset.yml
EXPOSE 7070
CMD ["./facade"]
