/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"github.com/golang/glog"
	"net/http"
	"time"

	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"

	"github.com/mbohlool/kcrdc/converter"
)

// Config contains the server (the webhook) cert and key.
type Config struct {
	CertFile   string
	KeyFile    string
	masterURL  string
	kubeconfig string
}

func (c *Config) addFlags() {
	flag.StringVar(&c.CertFile, "tls-cert-file", c.CertFile, ""+
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert).")
	flag.StringVar(&c.KeyFile, "tls-private-key-file", c.KeyFile, ""+
		"File containing the default x509 private key matching --tls-cert-file.")
	flag.StringVar(&c.kubeconfig, "kubeconfig", c.kubeconfig,
		"Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&c.masterURL, "master", c.masterURL,
		"The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func main() {
	// printServerCert("default", "kcrdc-webhook-service")
	var config Config
	config.addFlags()
	flag.Parse()
	glog.V(2).Info("Starting webhook ...")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(config.masterURL, config.kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}
	kubeClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	schemaCollectorController := NewCrdSchemaController(kubeInformerFactory.Apiextensions().V1beta1().CustomResourceDefinitions(), kubeClient.ApiextensionsV1beta1())

	kubeInformerFactory.Start(stopCh)
	go schemaCollectorController.Run(stopCh)

	http.Handle("/crdconvert", converter.NewDeclarativeConverterHandler(schemaCollectorController))
	server := &http.Server{
		Addr:      ":443",
		TLSConfig: configTLS(config),
	}
	server.ListenAndServeTLS("", "")
}

func configTLS(config Config) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		glog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
		// TODO: uses mutual tls after we agree on what cert the apiserver should use.
		// ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}
