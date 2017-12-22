package envoy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/envoyproxy/go-control-plane/api"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
	jsonnet "github.com/google/go-jsonnet"
	"github.com/kyessenov/envoymesh/model"
)

type output struct {
	Listeners []interface{} `json:"listeners"`
	Routes    []interface{} `json:"routes"`
	Clusters  []interface{} `json:"clusters"`
}

// Compiler represents a repeatedly executed compilation job
type Compiler struct {
	count int

	// TODO: sharing of VM and script AST
	vm     *jsonnet.VM
	script string

	// inputs
	id        string
	domain    string
	services  []*model.Service
	instances []model.ServiceInstance

	// outputs
	listeners []proto.Message
	routes    []proto.Message
	clusters  []proto.Message
	endpoints []proto.Message
}

func NewCompiler(name, namespace, suffix string) (*Compiler, error) {
	glog.Infof("prepare jsonnet VM")
	vm := jsonnet.MakeVM()
	content, err := ioutil.ReadFile("envoy.jsonnet")
	if err != nil {
		return nil, err
	}
	return &Compiler{
		vm:        vm,
		script:    string(content),
		id:        fmt.Sprintf("%s.%s.pod.%s", name, namespace, suffix),
		domain:    fmt.Sprintf("%s.svc.%s", namespace, suffix),
		listeners: make([]proto.Message, 0),
		routes:    make([]proto.Message, 0),
		clusters:  make([]proto.Message, 0),
	}, nil
}

// Update re-compiles if necessary and returns true only then
func (g *Compiler) Update(services []*model.Service, instances []model.ServiceInstance) (bool, error) {
	if reflect.DeepEqual(services, g.services) && reflect.DeepEqual(instances, g.instances) {
		return false, nil
	}

	g.count++
	g.services = services
	g.instances = instances

	servicesJSON, err := json.Marshal(g.services)
	if err != nil {
		return false, err
	}
	instancesJSON, err := json.Marshal(g.instances)
	if err != nil {
		return false, err
	}

	glog.Infof("generating snapshot %d for %s", g.count, g.id)
	g.vm.TLACode("services", string(servicesJSON))
	g.vm.TLACode("instances", string(instancesJSON))
	g.vm.TLAVar("domain", g.domain)
	in, err := g.vm.EvaluateSnippet("envoy.jsonnet", g.script)
	if err != nil {
		return true, err
	}
	glog.Infof("finished evaluation %d for %s", g.count, g.id)

	out := output{}
	if err := json.Unmarshal([]byte(in), &out); err != nil {
		return true, err
	}

	g.clusters = make([]proto.Message, 0)
	for _, cluster := range out.Clusters {
		l := api.Cluster{}
		s, _ := json.Marshal(cluster)
		if err := jsonpb.UnmarshalString(string(s), &l); err != nil {
			return true, err
		}
		g.clusters = append(g.clusters, &l)
	}

	g.routes = make([]proto.Message, 0)
	for _, route := range out.Routes {
		r := api.RouteConfiguration{}
		s, _ := json.Marshal(route)
		if err := jsonpb.UnmarshalString(string(s), &r); err != nil {
			return true, err
		}
		g.routes = append(g.routes, &r)
	}

	g.listeners = make([]proto.Message, 0)
	for _, listener := range out.Listeners {
		l := api.Listener{}
		s, _ := json.Marshal(listener)
		if err := jsonpb.UnmarshalString(string(s), &l); err != nil {
			return true, err
		}
		g.listeners = append(g.listeners, &l)
	}

	return true, nil
}

func (g *Compiler) UpdateEndpoints(controller model.ServiceDiscovery) bool {
	endpoints := make([]proto.Message, 0, len(g.clusters))
	for _, msg := range g.clusters {
		cluster := msg.(*api.Cluster)
		// note that EDS presents service name instead of cluster name here
		if cluster.EdsClusterConfig != nil {
			hostname, ports, labelcols := model.ParseServiceKey(cluster.EdsClusterConfig.ServiceName)
			instances, err := controller.Instances(hostname, ports.GetNames(), labelcols)
			if err != nil {
				glog.Warning(err)
			}
			addresses := make([]*api.LbEndpoint, 0, len(instances))
			for _, instance := range instances {
				addresses = append(addresses, &api.LbEndpoint{
					Endpoint: &api.Endpoint{
						Address: &api.Address{
							Address: &api.Address_SocketAddress{
								SocketAddress: &api.SocketAddress{
									Address:       instance.Endpoint.Address,
									PortSpecifier: &api.SocketAddress_PortValue{PortValue: uint32(instance.Endpoint.Port)},
								},
							},
						},
					},
				})
			}
			endpoints = append(endpoints, &api.ClusterLoadAssignment{
				ClusterName: cluster.EdsClusterConfig.ServiceName,
				Endpoints:   []*api.LocalityLbEndpoints{{LbEndpoints: addresses}}})
		}
	}

	if reflect.DeepEqual(g.endpoints, endpoints) {
		return false
	}

	g.endpoints = endpoints
	return true
}

func (g *Compiler) Snapshot(version int) cache.Snapshot {
	return cache.NewSnapshot(fmt.Sprintf("%d", version),
		g.endpoints, g.clusters, g.routes, g.listeners)
}
