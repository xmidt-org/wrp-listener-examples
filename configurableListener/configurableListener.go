// SPDX-FileCopyrightText: 2019 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha1"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/metrics/provider"
	"github.com/justinas/alice"
	"github.com/spf13/viper"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/bascule/basculehttp"
	webhook "github.com/xmidt-org/wrp-listener"
	"github.com/xmidt-org/wrp-listener/hashTokenFactory"
	secretGetter "github.com/xmidt-org/wrp-listener/secret"
	"github.com/xmidt-org/wrp-listener/webhookClient"
	"go.uber.org/zap"
)

const (
	applicationName = "configurableListener"
)

type Config struct {
	AuthHeader                  string
	AuthDelimiter               string
	WebhookRequest              webhook.W
	WebhookRegistrationURL      string
	WebhookTimeout              time.Duration
	WebhookRegistrationInterval time.Duration
	Port                        string
	Endpoint                    string
	ResponseCode                int
}

func main() {
	// load configuration with viper
	v := viper.New()
	v.AddConfigPath(".")
	v.SetConfigName(applicationName)
	err := v.ReadInConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read in viper config: %v\n", err.Error())
		os.Exit(1)
	}
	config := new(Config)
	err = v.Unmarshal(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal config: %v\n", err.Error())
		os.Exit(1)
	}

	// use constant secret for hash
	secretGetter := secretGetter.NewConstantSecret(config.WebhookRequest.Config.Secret)

	// set up the middleware
	htf, err := hashTokenFactory.New("sha1", sha1.New, secretGetter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup hash token factory: %v\n", err.Error())
		os.Exit(1)
	}
	authConstructor := basculehttp.NewConstructor(
		basculehttp.WithTokenFactory("sha1", htf),
		basculehttp.WithHeaderName(config.AuthHeader),
		basculehttp.WithHeaderDelimiter(config.AuthDelimiter),
	)
	handler := alice.New(authConstructor)

	// set up the registerer
	basicConfig := webhookClient.BasicConfig{
		Timeout:         config.WebhookTimeout,
		RegistrationURL: config.WebhookRegistrationURL,
		Request:         config.WebhookRequest,
	}
	// This Basic Auth credentials intended to be used for local testing purposes.
	// Change this.
	acquirer, err := acquire.NewFixedAuthAcquirer("Basic dXNlcjpwYXNz")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create fixed auth: %v\n", err.Error())
		os.Exit(1)
	}
	registerer, err := webhookClient.NewBasicRegisterer(acquirer, secretGetter, basicConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup registerer: %v\n", err.Error())
		os.Exit(1)
	}
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup logger: %v\n", err.Error())
		os.Exit(1)
	}
	periodicRegisterer, err := webhookClient.NewPeriodicRegisterer(registerer, 55*time.Second, logger, webhookClient.NewMeasures(provider.NewDiscardProvider()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup periodic registerer: %v\n", err.Error())
		os.Exit(1)
	}
	// start the registerer
	periodicRegisterer.Start()

	// start listening
	http.Handle(config.Endpoint, handler.ThenFunc(returnStatus(config.ResponseCode)))
	err = http.ListenAndServe(config.Port, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serving http requests: %v\n", err.Error())
		os.Exit(1)
	}
}

func returnStatus(code int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("received http request")
		w.WriteHeader(code)
	}
}
