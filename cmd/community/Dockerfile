FROM debian:bookworm-slim
LABEL maintainer="signoz"
WORKDIR /root

ARG OS="linux"
ARG TARGETARCH

RUN apt-get update && \
	apt-get install -y --no-install-recommends ca-certificates && \
	rm -rf /var/lib/apt/lists/*


COPY ./target/${OS}-${TARGETARCH}/signoz-community /root/signoz
COPY ./templates/email /root/templates
COPY frontend/build/ /etc/signoz/web/

RUN chmod 755 /root /root/signoz

ENTRYPOINT ["./signoz", "server"]
