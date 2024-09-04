FROM --platform=$BUILDPLATFORM golang:alpine AS builder

WORKDIR /apps

RUN apk add --no-cache git

RUN git clone https://github.com/wy2919/go-network-monitor.git .

RUN go mod init main && go mod tidy

ARG TARGETOS
ARG TARGETARCH

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /apps/main /apps/main.go

FROM --platform=$TARGETPLATFORM alpine:latest

WORKDIR /apps

COPY --from=builder /apps/main .

RUN apk update && apk add --no-cache openssh-client sshpass dbus tzdata && chmod +x main

RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

ARG TARGETPLATFORM

RUN if [ "$TARGETPLATFORM" = "linux/amd64" ]; then \
        mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2; \
    elif [ "$TARGETPLATFORM" = "linux/arm64" ]; then \
        echo "arm平台"; \
    fi

ENV INTERVAL=30 \
    PARDON=600 \
    NAME="" \
    HOST="" \
    MODEL=1 \
    GB=1000000 \
    INTERFACE="eth0" \
    WXKEY="" \
    SHUTDOWN="no" \
    SHUTDOWNTYPE="dbus" \
    SSHHOST="" \
    SSHPWD="" \
    SSHPORT=22 \
    SMTPHOST="smtp.qq.com:587" \
    SMTPEMAIL="" \
    SMTPPWD=""

CMD ./main \
  -interval $INTERVAL \
  -pardon $PARDON \
  -name $NAME \
  -host $HOST \
  -model $MODEL \
  -gb $GB \
  -interface $INTERFACE \
  -wxKey $WXKEY \
  -shutdown $SHUTDOWN \
  -shutdownType $SHUTDOWNTYPE \
  -sshHost $SSHHOST \
  -sshPwd $SSHPWD \
  -sshPort $SSHPORT \
  -smtpHost $SMTPHOST \
  -smtpEmail $SMTPEMAIL \
  -smtpPwd $SMTPPWD
