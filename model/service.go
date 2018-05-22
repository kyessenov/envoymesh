// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file describes the abstract model of services (and their instances) as
// represented in Istio. This model is independent of the underlying platform
// (Kubernetes, Mesos, etc.). Platform specific adapters found populate the
// model object with various fields, from the metadata found in the platform.
// The platform independent proxy code uses the representation in the model to
// generate the configuration files for the Layer 7 proxy sidecar. The proxy
// code is specific to individual proxy implementations

package model

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	proxyconfig "istio.io/api/proxy/v1/config"
)

// Service describes an Istio service (e.g., catalog.mystore.com:8080)
// Each service has a fully qualified domain name (FQDN) and one or more
// ports where the service is listening for connections. *Optionally*, a
// service can have a single load balancer/virtual IP address associated
// with it, such that the DNS queries for the FQDN resolves to the virtual
// IP address (a load balancer IP).
//
// E.g., in kubernetes, a service foo is associated with
// foo.default.svc.cluster.local hostname, has a virtual IP of 10.0.1.1 and
// listens on ports 80, 8080
type Service struct {
	// Hostname of the service, e.g. "catalog.mystore.com"
	Hostname string `json:"hostname"`

	// Address specifies the service IPv4 address of the load balancer
	Address string `json:"address,omitempty"`

	// Ports is the set of network ports where the service is listening for
	// connections
	Ports PortList `json:"ports"`

	// ExternalName is only set for external services and holds the external
	// service DNS name.  External services are name-based solution to represent
	// external service instances as a service inside the cluster.
	ExternalName string `json:"external,omitempty"`

	// ServiceAccounts specifies the service accounts that run the service.
	ServiceAccounts []string `json:"serviceaccounts,omitempty"`

	// LoadBalancingDisabled indicates that no load balancing should be done for this service.
	LoadBalancingDisabled bool `json:"-"`
}

// Port represents a network port where a service is listening for
// connections. The port should be annotated with the type of protocol
// used by the port.
type Port struct {
	// Name ascribes a human readable name for the port object. When a
	// service has multiple ports, the name field is mandatory
	Name string `json:"name"`

	// Port number where the service can be reached. Does not necessarily
	// map to the corresponding port numbers for the instances behind the
	// service. See NetworkEndpoint definition below.
	Port int `json:"port"`

	// Protocol to be used for the port.
	Protocol Protocol `json:"protocol"`

	// In combine with the mesh's AuthPolicy, controls authentication for
	// Envoy-to-Envoy communication.
	// This value is extracted from service annotation.
	AuthenticationPolicy proxyconfig.AuthenticationPolicy `json:"authentication_policy"`
}

// PortList is a set of ports
type PortList []*Port

// Protocol defines network protocols for ports
type Protocol string

const (
	// ProtocolGRPC declares that the port carries gRPC traffic
	ProtocolGRPC Protocol = "GRPC"
	// ProtocolHTTPS declares that the port carries HTTPS traffic
	ProtocolHTTPS Protocol = "HTTPS"
	// ProtocolHTTP2 declares that the port carries HTTP/2 traffic
	ProtocolHTTP2 Protocol = "HTTP2"
	// ProtocolHTTP declares that the port carries HTTP/1.1 traffic.
	// Note that HTTP/1.0 or earlier may not be supported by the proxy.
	ProtocolHTTP Protocol = "HTTP"
	// ProtocolTCP declares the the port uses TCP.
	// This is the default protocol for a service port.
	ProtocolTCP Protocol = "TCP"
	// ProtocolUDP declares that the port uses UDP.
	// Note that UDP protocol is not currently supported by the proxy.
	ProtocolUDP Protocol = "UDP"
	// ProtocolMongo declares that the port carries mongoDB traffic
	ProtocolMongo Protocol = "Mongo"
	// ProtocolRedis declares that the port carries redis traffic
	ProtocolRedis Protocol = "Redis"
)

// IsHTTP is true for protocols that use HTTP as transport protocol
func (p Protocol) IsHTTP() bool {
	switch p {
	case ProtocolHTTP, ProtocolHTTP2, ProtocolGRPC:
		return true
	default:
		return false
	}
}

// Labels is a non empty set of arbitrary strings. Each version of a service can
// be differentiated by a unique set of labels associated with the version. These
// labels are assigned to all instances of a particular service version. For
// example, lets say catalog.mystore.com has 2 versions v1 and v2. v1 instances
// could have labels gitCommit=aeiou234, region=us-east, while v2 instances could
// have labels name=kittyCat,region=us-east.
type Labels map[string]string

// LabelsCollection is a collection of labels used for comparing labels against a
// collection of labels
type LabelsCollection []Labels

// Endpoint is a network listener descriptor
type Endpoint struct {
	IP       string   `json:"ip"`
	Port     int      `json:"port"`
	Protocol Protocol `json:"protocol"`

	// Used by EDS
	UID string `json:"uid"`
}

// Instance is a workload descriptor
type Instance struct {
	Endpoints []Endpoint `json:"endpoints"`
	Labels    Labels     `json:"labels"`
	UID       string     `json:"uid"`
}

// ServiceDiscovery enumerates Istio service instances.
type ServiceDiscovery interface {
	// Services list declarations of all services in the system
	Services() []*Service

	// Instances ...
	Instances() map[string][]Endpoint

	// Workload ...
	Workload(id string) (Instance, error)
}

// ServiceAccounts exposes Istio service accounts
type ServiceAccounts interface {
	// GetIstioServiceAccounts returns a list of service accounts looked up from
	// the specified service hostname and ports.
	GetIstioServiceAccounts(hostname string, ports []string) []string
}

// SubsetOf is true if the tag has identical values for the keys
func (t Labels) SubsetOf(that Labels) bool {
	for k, v := range t {
		if that[k] != v {
			return false
		}
	}
	return true
}

// Equals returns true if the labels are identical
func (t Labels) Equals(that Labels) bool {
	if t == nil {
		return that == nil
	}
	if that == nil {
		return t == nil
	}
	return t.SubsetOf(that) && that.SubsetOf(t)
}

// HasSubsetOf returns true if the input labels are a super set of one labels in a
// collection or if the tag collection is empty
func (labels LabelsCollection) HasSubsetOf(that Labels) bool {
	if len(labels) == 0 {
		return true
	}
	for _, tag := range labels {
		if tag.SubsetOf(that) {
			return true
		}
	}
	return false
}

// GetNames returns port names
func (ports PortList) GetNames() []string {
	names := make([]string, 0, len(ports))
	for _, port := range ports {
		names = append(names, port.Name)
	}
	return names
}

// Get retrieves a port declaration by name
func (ports PortList) Get(name string) (*Port, bool) {
	for _, port := range ports {
		if port.Name == name {
			return port, true
		}
	}
	return nil, false
}

// GetByPort retrieves a port declaration by port value
func (ports PortList) GetByPort(num int) (*Port, bool) {
	for _, port := range ports {
		if port.Port == num {
			return port, true
		}
	}
	return nil, false
}

// External predicate checks whether the service is external
func (s *Service) External() bool {
	return s.ExternalName != ""
}

func (t Labels) String() string {
	labels := make([]string, 0, len(t))
	for k, v := range t {
		if len(v) > 0 {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		} else {
			labels = append(labels, k)
		}
	}
	sort.Strings(labels)

	var buffer bytes.Buffer
	var first = true
	for _, label := range labels {
		if !first {
			buffer.WriteString(",")
		} else {
			first = false
		}
		buffer.WriteString(label)
	}
	return buffer.String()
}

// ParseLabelsString extracts labels from a string
func ParseLabelsString(s string) Labels {
	pairs := strings.Split(s, ",")
	tag := make(map[string]string, len(pairs))

	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) > 1 {
			tag[kv[0]] = kv[1]
		} else {
			tag[kv[0]] = ""
		}
	}
	return tag
}
