package envoy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/golang/glog"
	jsonnet "github.com/google/go-jsonnet"
	"github.com/kyessenov/envoymesh/model"
)

type output struct {
	Listeners []interface{} `json:"listeners"`
	Routes    []interface{} `json:"routes"`
	Clusters  []interface{} `json:"clusters"`
	Endpoints []interface{} `json:"endpoints"`
}

// Compiler represents a repeatedly executed compilation job
type Compiler struct {
	count int

	// TODO: sharing of VM and script AST
	vm     *jsonnet.VM
	script string

	// inputs
	uid       string
	domain    string
	services  []*model.Service
	instance  model.Instance
	instances map[string][]model.Endpoint

	// outputs
	listeners []cache.Resource
	routes    []cache.Resource
	clusters  []cache.Resource
	endpoints []cache.Resource
}

// NewCompiler instantiates a jsonnet compiler
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
		uid:       fmt.Sprintf("kubernetes://%s.%s", name, namespace),
		domain:    fmt.Sprintf("%s.svc.%s", namespace, suffix),
		listeners: make([]cache.Resource, 0),
		routes:    make([]cache.Resource, 0),
		clusters:  make([]cache.Resource, 0),
		endpoints: make([]cache.Resource, 0),
	}, nil
}

// Update re-compiles if necessary and returns true only then
func (g *Compiler) Update(services []*model.Service, instance model.Instance, instances map[string][]model.Endpoint) (bool, error) {
	if reflect.DeepEqual(services, g.services) && reflect.DeepEqual(instance, g.instance) && reflect.DeepEqual(instances, g.instances) {
		return false, nil
	}

	g.count++
	g.services = services
	g.instance = instance
	g.instances = instances

	servicesJSON, err := json.Marshal(g.services)
	if err != nil {
		return false, err
	}
	instanceJSON, err := json.Marshal(g.instance)
	if err != nil {
		return false, err
	}
	instancesJSON, err := json.Marshal(g.instances)
	if err != nil {
		return false, err
	}

	glog.Infof("generating snapshot %d for %s", g.count, g.uid)
	g.vm.TLACode("services", string(servicesJSON))
	g.vm.TLACode("instance", string(instanceJSON))
	g.vm.TLACode("instances", string(instancesJSON))
	g.vm.TLAVar("domain", g.domain)
	in, err := g.vm.EvaluateSnippet("envoy.jsonnet", g.script)
	if err != nil {
		return true, err
	}
	glog.Infof("finished evaluation %d for %s", g.count, g.uid)

	out := output{}
	if err := json.Unmarshal([]byte(in), &out); err != nil {
		return true, err
	}

	g.clusters = make([]cache.Resource, 0)
	for _, cluster := range out.Clusters {
		l := v2.Cluster{}
		s, _ := json.Marshal(cluster)
		if err := jsonpb.UnmarshalString(string(s), &l); err != nil {
			return true, err
		}
		g.clusters = append(g.clusters, &l)
	}

	g.routes = make([]cache.Resource, 0)
	for _, route := range out.Routes {
		r := v2.RouteConfiguration{}
		s, _ := json.Marshal(route)
		if err := jsonpb.UnmarshalString(string(s), &r); err != nil {
			return true, err
		}
		g.routes = append(g.routes, &r)
	}

	g.listeners = make([]cache.Resource, 0)
	for _, listener := range out.Listeners {
		l := v2.Listener{}
		s, _ := json.Marshal(listener)
		if err := jsonpb.UnmarshalString(string(s), &l); err != nil {
			return true, err
		}
		g.listeners = append(g.listeners, &l)
	}

	g.endpoints = make([]cache.Resource, 0)
	for _, endpoint := range out.Endpoints {
		e := v2.ClusterLoadAssignment{}
		s, _ := json.Marshal(endpoint)
		if err := jsonpb.UnmarshalString(string(s), &e); err != nil {
			return true, err
		}
		g.endpoints = append(g.endpoints, &e)
	}

	return true, nil
}

// Snapshot ...
func (g *Compiler) Snapshot(version int) cache.Snapshot {
	return cache.NewSnapshot(fmt.Sprintf("%d", version),
		g.endpoints, g.clusters, g.routes, g.listeners)
}
