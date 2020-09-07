FROM alpine
RUN apk update && apk add libpcap
COPY sniffit /usr/bin/sniffit
ENTRYPOINT ["/usr/bin/sniffit"]