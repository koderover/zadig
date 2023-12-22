FROM alpine/git:v2.36.3

COPY myapp-2 /myapp-2

RUN apk add --no-cache curl

# This would be nicer as `nobody:nobody` but distroless has no such entries.
USER 65535:65535
ENV HOME /

ENTRYPOINT ["/myapp-2"]
