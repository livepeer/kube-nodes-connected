package main

import (
	"context"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	ff "github.com/peterbourgon/ff/v3"
	v1 "k8s.io/api/core/v1"
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
	address := fs.String("address", "0.0.0.0:80", "Address to listen on")
	namespace := fs.String("namespace", "default", "Kubernetes namespace to monitor")
	ownPodIp := fs.String("own-pod-ip", "", "IP address of this node (for skipping)")

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
		glog.Infof("Listening on %s", *address)
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

	// Not immediately known, but we'll find it in the endpoints list
	var ownNodeName string

	for {
		glog.V(6).Infof("Starting endpoints query")
		endpoints, err := clientset.CoreV1().Endpoints(*namespace).Get(context.TODO(), *endpointsName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			glog.Fatalf("Endpoints %s not found in default namespace", *endpointsName)
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			glog.Fatalf("Error getting endpoints %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		}
		glog.V(6).Infof("Finished endpoints query, starting HTTP requests")

		var wg sync.WaitGroup

		start := time.Now()
		successes := 0
		total := 0

		for _, subset := range endpoints.Subsets {
			for _, address := range subset.Addresses {
				if address.IP == *ownPodIp {
					ownNodeName = *address.NodeName
					continue
				}
				total += 1
				wg.Add(1)
				go func(addr v1.EndpointAddress) {
					var netClient = &http.Client{
						Timeout: time.Second * 10,
					}
					reqUrl := fmt.Sprintf("http://%s/", addr.IP)
					glog.V(6).Infof("Starting request to %s: %s", *addr.NodeName, reqUrl)
					resp, err := netClient.Get(reqUrl)
					if err != nil {
						glog.Errorf("NODE_TIMEOUT localNode=%s remoteNode=%s localIP=%s remoteIP=%s", ownNodeName, *addr.NodeName, *ownPodIp, addr.IP)
						wg.Done()
						return
					}
					defer resp.Body.Close()
					_, err = io.ReadAll(resp.Body)
					if err != nil {
						glog.Errorf("BODY_READ_FAILURE localNode=%s remoteNode=%s localIP=%s remoteIP=%s", ownNodeName, *addr.NodeName, *ownPodIp, addr.IP)
						wg.Done()
						return
					}
					if resp.StatusCode != 200 {
						glog.Errorf("UNEXPECTED_STATUS localNode=%s remoteNode=%s localIP=%s remoteIP=%s", ownNodeName, *addr.NodeName, *ownPodIp, addr.IP)
						wg.Done()
						return
					}
					successes += 1
					wg.Done()
				}(address)
			}
		}

		wg.Wait()
		took := time.Since(start).Seconds()
		glog.Infof("Got %d/%d responses in %.2fs", successes, total, took)
		glog.V(6).Infof("Sleeping for 5 seconds")

		time.Sleep(5 * time.Second)
	}
}
