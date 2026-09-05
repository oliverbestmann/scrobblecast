FROM mirror.gcr.io/golang:1.27 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/scrobblecast .


FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /data

COPY --from=build /out/scrobblecast /usr/local/bin/scrobblecast

ENTRYPOINT ["/usr/local/bin/scrobblecast"]
CMD ["-db", "/data/scrobblecast.db"]
