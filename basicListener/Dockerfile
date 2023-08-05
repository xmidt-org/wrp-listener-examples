# SPDX-FileCopyrightText: 2019 Comcast Cable Communications Management, LLC
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.18

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /wrp-listener-example

EXPOSE 7100

CMD ["/wrp-listener-example"]
