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
RUN apt update && apt install -y fontconfig
COPY --from=build /app/cmd/geneos/geneos /bin
RUN useradd -ms /bin/bash geneos
USER geneos
WORKDIR /home/geneos
CMD [ "bash" ]
