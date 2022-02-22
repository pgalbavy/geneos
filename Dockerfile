FROM golang AS build
WORKDIR /app
COPY go.mod /app/
COPY go.sum /app/

COPY cmd /app/cmd
COPY pkg /app/pkg

WORKDIR /app/cmd/geneos
RUN go build

FROM debian
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/cmd/geneos/geneos /bin
RUN apt update && apt install -y fontconfig
RUN useradd -ms /bin/bash geneos
USER geneos
WORKDIR /home/geneos
EXPOSE 8080/tcp
# ENV ITRS_HOME=/home/geneos
RUN geneos init
CMD [ "bash" ]
