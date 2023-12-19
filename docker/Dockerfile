FROM ubuntu as build


RUN apt update && apt install -y curl build-essential libsqlite3-dev

RUN curl -o go1.21.5.linux-amd64.tar.gz -L https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
ENV PATH="${PATH}:/usr/local/go/bin"

WORKDIR /root
COPY go.* ./
COPY pkg ./pkg
RUN go build -o main ./pkg/...

FROM ubuntu

RUN groupadd user && \
    useradd -m -s /sbin/nologin -g user user

RUN mkdir -p /workspace/db
RUN chown -R user:user /workspace

COPY --from=build /root/main /workspace/main

WORKDIR /workspace
ENTRYPOINT [ "./main" ]