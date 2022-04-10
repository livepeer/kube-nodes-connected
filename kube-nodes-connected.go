package main

import (
	"context"
	"flag"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"
	ff "github.com/peterbourgon/ff/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

func main() {
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet("kube-nodes-connected", flag.ExitOnError)

	verbosity := fs.String("v", "", "Log verbosity.  {4|5|6}")
	endpointsName := fs.String("endpoints", "", "Endpoints object to monitor for peer services")
	address := fs.String("address", "127.0.0.1:80", "Address to listen on")
	namespace := fs.String("namespace", "default", "Kubernetes namespace to monitor")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("KUBE_NODES"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	vFlag.Value.Set(*verbosity)

	// Start HTTP server

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		glog.V(6).Infof("got request")
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	go func() {
		glog.Infof("Listening on %s", address)
		log.Fatal(http.ListenAndServe(*address, nil))
	}()

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {
		// Examples for error handling:
		// - Use helper functions e.g. errors.IsNotFound()
		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
		endpoints, err := clientset.CoreV1().Endpoints(*namespace).Get(context.TODO(), *endpointsName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			glog.Fatalf("Endpoints %s not found in default namespace", *endpointsName)
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			glog.Fatalf("Error getting endpoints %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			glog.Infof("Endpoints: %v", endpoints)
		}

		time.Sleep(60 * time.Second)
	}
}
