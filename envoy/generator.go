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

type Generator struct {
	count      int
	controller *kube.Controller
	cache      cache.SnapshotCache
	services   []*model.Service

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
	g.controller.RegisterInstanceHandler(g.UpdateInstances)

	// callback: registering a new node group (on a different loop)
	g.cache = cache.NewSnapshotCache(true, g, g)

	return g, nil
}

func (g *Generator) Run(stop <-chan struct{}) {
	g.controller.Run(stop)
	<-stop
}

func (g *Generator) ID(node *core.Node) string {
	return node.GetId()
}
func (g *Generator) Infof(format string, args ...interface{})  { glog.Infof(format, args...) }
func (g *Generator) Errorf(format string, args ...interface{}) { glog.Errorf(format, args...) }
func (g *Generator) Cache() cache.Cache                        { return g.cache }

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
func (g *Generator) OnStreamOpen(int64, string)                                           {}
func (g *Generator) OnStreamClosed(int64)                                                 {}
func (g *Generator) OnStreamResponse(int64, *v2.DiscoveryRequest, *v2.DiscoveryResponse)  {}
func (g *Generator) OnFetchRequest(req *v2.DiscoveryRequest)                              {}
func (g *Generator) OnFetchResponse(req *v2.DiscoveryRequest, resp *v2.DiscoveryResponse) {}

func (g *Generator) UpdateNode(key string) {
	compiler := g.nodes[key]
	instances, err := g.controller.WorkloadInstances(string(key))
	if err != nil {
		glog.Warning(err)
	}
	if instances == nil {
		instances = []model.ServiceInstance{}
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Service.Hostname < instances[j].Service.Hostname ||
			(instances[i].Service.Hostname == instances[j].Service.Hostname && instances[i].Endpoint.Port < instances[j].Endpoint.Port)
	})

	updated, err := compiler.Update(g.services, instances)
	if err != nil {
		glog.Warning(err)
	}

	updatedEndpoints := compiler.UpdateEndpoints(g.controller)

	if updated || updatedEndpoints {
		g.count++
		glog.Infof("update node %v (updated=%t, updatedEndpoints=%t, count=%d)", key, updated, updatedEndpoints, g.count)
		g.cache.SetSnapshot(key, compiler.Snapshot(g.count))
	}
}

func (g *Generator) UpdateServices(*model.Service, model.Event) {
	// reload services
	services, err := g.controller.Services()
	if err != nil {
		glog.Warning(err)
	}
	if services == nil {
		services = []*model.Service{}
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Hostname < services[j].Hostname })

	if reflect.DeepEqual(services, g.services) {
		return
	}

	glog.Infof("update services (services=%d)", len(services))
	g.services = services
	g.Update()
}

func (g *Generator) UpdateInstances(*model.ServiceInstance, model.Event) {
	g.Update()
}

func (g *Generator) Update() {
	for key := range g.nodes {
		g.UpdateNode(key)
	}
}
