# The md-to-tex service on a TeX Live image (srd010-service R5). The base
# carries latexmk and xelatex; IEEEtran is added because the small scheme
# does not ship the publisher classes. The binary is built in a Go stage so
# the image expects no network at run time.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/md-to-tex ./cmd/md-to-tex

FROM texlive/texlive:latest-small
# The small scheme already carries natbib, booktabs, url, fontspec, and the
# tools bundle tabularx lives in; IEEEtran is the one class it lacks.
RUN tlmgr install ieeetran && kpsewhich IEEEtran.cls
COPY --from=build /out/md-to-tex /usr/local/bin/md-to-tex
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/md-to-tex", "serve", "--listen", ":8090"]
