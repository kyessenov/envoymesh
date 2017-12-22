package envoy

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/envoyproxy/go-control-plane/api"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/golang/glog"
	"github.com/kyessenov/envoymesh/kube"
	"github.com/kyessenov/envoymesh/model"
)

type Generator struct {
	count      int
	controller *kube.Controller
	cache      cache.Cache
	services   []*model.Service

	nodes map[cache.Key]*Compiler
}

const (
	suffix = "cluster.local"
)

func NewKubeGenerator(kubeconfig string) (*Generator, error) {
	g := &Generator{
		nodes: make(map[cache.Key]*Compiler),
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
	g.cache = cache.NewSimpleCache(g, g.RegisterNodeGroup)

	return g, nil
}

func (g *Generator) Run(stop <-chan struct{}) {
	g.controller.Run(stop)
	<-stop
}

func (g *Generator) Hash(node *api.Node) (cache.Key, error) {
	return cache.Key(node.GetId()), nil
}

func (g *Generator) ConfigWatcher() cache.ConfigWatcher {
	return g.cache
}

func (g *Generator) RegisterNodeGroup(key cache.Key) {
	// move the task to single threaded queue
	g.controller.QueueSchedule(func() {
		if _, exists := g.nodes[key]; !exists {
			glog.Infof("register node group %v", key)
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
			glog.Infof("first update for node %v", key)
			g.UpdateNode(key, compiler)
		}
	})
}

func (g *Generator) UpdateNode(key cache.Key, compiler *Compiler) {
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
	for key, compiler := range g.nodes {
		g.UpdateNode(key, compiler)
	}
}
