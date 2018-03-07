package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"

	"github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	"github.com/golang/glog"
	"github.com/kyessenov/envoymesh/envoy"
	"google.golang.org/grpc"
)

func main() {
	flag.Parse()
	stop := make(chan struct{})

	generator, err := envoy.NewKubeGenerator(kubeconfig)
	if err != nil {
		glog.Fatal(err)
	}

	srv := server.NewServer(generator.Cache(), generator)
	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
	}
	v2.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)

	go generator.Run(stop)

	// expose profiling endpoint
	go http.ListenAndServe(":15005", nil)

	if err = grpcServer.Serve(lis); err != nil {
		glog.Error(err)
	}
}

var (
	kubeconfig string
	port       int
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Use a Kubernetes configuration file instead of in-cluster configuration")
	flag.IntVar(&port, "port", 8080, "ADS port")
}
