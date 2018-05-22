package envoy

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/golang/glog"
	"github.com/kyessenov/envoymesh/kube"
	"github.com/kyessenov/envoymesh/model"
)

// Generator produces envoy configs
type Generator struct {
	count      int
	controller model.Controller
	cache      cache.SnapshotCache
	services   []*model.Service
	instances  map[string][]model.Endpoint

	nodes map[string]*Compiler
}

const (
	suffix = "cluster.local"
)

func NewKubeGenerator(kubeconfig string) (*Generator, error) {
	g := &Generator{
		nodes: make(map[string]*Compiler),
	}

	_, client, err := kube.CreateInterface(kubeconfig)
	if err != nil {
		return nil, err
	}

	options := kube.ControllerOptions{ResyncPeriod: 60 * time.Second, DomainSuffix: suffix}
	g.controller = kube.NewController(client, options)

	// callback: service modification
	g.controller.RegisterServiceHandler(g.UpdateServices)

	// callback: endpoint modification
	g.controller.RegisterEndpointHandler(g.UpdateInstances)

	// callback: registering a new node group (on a different loop)
	g.cache = cache.NewSnapshotCache(true, g, g)

	return g, nil
}

// Run ...
func (g *Generator) Run(stop <-chan struct{}) {
	g.controller.Run(stop)
	<-stop
}

// ID ...
func (g *Generator) ID(node *core.Node) string {
	return node.GetId()
}

// Infof ...
func (g *Generator) Infof(format string, args ...interface{}) { glog.Infof(format, args...) }

// Errorf ...
func (g *Generator) Errorf(format string, args ...interface{}) { glog.Errorf(format, args...) }

// Cache ...
func (g *Generator) Cache() cache.Cache { return g.cache }

// OnStreamRequest ...
func (g *Generator) OnStreamRequest(id int64, req *v2.DiscoveryRequest) {
	// move the task to single threaded queue
	g.controller.QueueSchedule(func() {
		key := g.ID(req.GetNode())
		if _, exists := g.nodes[key]; !exists {
			parts := strings.Split(string(key), "/")
			name, namespace := "", "default"
			switch len(parts) {
			case 1:
				// name only, no namespace
				name = parts[0]
			case 2:
				// namespace and name
				name, namespace = parts[1], parts[0]
			}
			compiler, err := NewCompiler(name, namespace, suffix)
			if err != nil {
				glog.Fatal(err)
			}
			g.nodes[key] = compiler
			g.UpdateNode(key)
		}
	})
}

// OnStreamOpen ...
func (g *Generator) OnStreamOpen(int64, string) {}

// OnStreamClosed ...
func (g *Generator) OnStreamClosed(int64) {}

// OnStreamResponse ...
func (g *Generator) OnStreamResponse(int64, *v2.DiscoveryRequest, *v2.DiscoveryResponse) {}

// OnFetchRequest ...
func (g *Generator) OnFetchRequest(req *v2.DiscoveryRequest) {}

// OnFetchResponse ...
func (g *Generator) OnFetchResponse(req *v2.DiscoveryRequest, resp *v2.DiscoveryResponse) {}

// UpdateNode ...
func (g *Generator) UpdateNode(key string) {
	compiler := g.nodes[key]
	instance, err := g.controller.Workload(string(key))
	if err != nil {
		glog.Warning(err)
	}
	sort.Slice(instance.Endpoints, func(i, j int) bool {
		return instance.Endpoints[i].IP < instance.Endpoints[j].IP ||
			(instance.Endpoints[i].IP == instance.Endpoints[j].IP && instance.Endpoints[i].Port < instance.Endpoints[j].Port)
	})

	updated, err := compiler.Update(g.services, instance, g.instances)
	if err != nil {
		glog.Warning(err)
	}

	if updated {
		g.count++
		glog.Infof("update node %v (updated=%t, count=%d)", key, updated, g.count)
		g.cache.SetSnapshot(key, compiler.Snapshot(g.count))
	}
}

// UpdateServices ...
func (g *Generator) UpdateServices() {
	// reload services
	services := g.controller.Services()
	if reflect.DeepEqual(services, g.services) {
		return
	}
	glog.Infof("update services (services=%d)", len(services))
	g.services = services
	g.Update()
}

// UpdateInstances ...
func (g *Generator) UpdateInstances() {
	instances := g.controller.Instances()
	if reflect.DeepEqual(instances, g.instances) {
		return
	}
	glog.Infof("update instances (instances=%d)", len(instances))
	g.instances = instances
	g.Update()
}

// Update ...
func (g *Generator) Update() {
	for key := range g.nodes {
		g.UpdateNode(key)
	}
}
