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
RUN useradd -ms /bin/bash geneos
USER geneos
WORKDIR /home/geneos
ENV ITRS_HOME=/home/geneos
CMD [ "bash" ]


