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

package inject

// NOTE: This tool only exists because kubernetes does not support
// dynamic/out-of-tree admission controller for transparent proxy
// injection. This file should be removed as soon as a proper kubernetes
// admission controller is written for istio.

import (
	"time"
)

// Defaults values for injecting istio proxy into kubernetes
// resources.
const (
	DefaultSidecarProxyUID = int64(1337)
	DefaultVerbosity       = 2
	DefaultHub             = "docker.io/istio"
	DefaultImagePullPolicy = "IfNotPresent"
)

const (
	// InitContainerName is the name for init container
	InitContainerName = "istio-init"

	// ProxyContainerName is the name for sidecar proxy container
	ProxyContainerName = "istio-proxy"

	enableCoreDumpContainerName = "enable-core-dump"
	enableCoreDumpImage         = "alpine"

	istioCertSecretPrefix = "istio."

	istioCertVolumeName        = "istio-certs"
	istioEnvoyConfigVolumeName = "istio-envoy"

	// ConfigMapKey should match the expected MeshConfig file name
	ConfigMapKey = "mesh"

	// InitializerConfigMapKey is the key into the initailizer ConfigMap data.
	InitializerConfigMapKey = "config"

	// DefaultResyncPeriod specifies how frequently to retrieve the
	// full list of watched resources for initialization.
	DefaultResyncPeriod = 30 * time.Second

	// DefaultInitializerName specifies the name of the initializer.
	DefaultInitializerName = "sidecar.initializer.istio.io"
)

// InitImageName returns the fully qualified image name for the istio
// init image given a docker hub and tag and debug flag
func InitImageName(hub string, tag string, _ bool) string {
	return hub + "/proxy_init:" + tag
}

// ProxyImageName returns the fully qualified image name for the istio
// proxy image given a docker hub and tag and whether to use debug or not.
func ProxyImageName(hub string, tag string, debug bool) string {
	if debug {
		return hub + "/proxy_debug:" + tag
	}
	return hub + "/proxy:" + tag
}

/*
func injectIntoSpec(p *Params, spec *v1.PodSpec, metadata *metav1.ObjectMeta) {
	// proxy initContainer 1.6 spec
	initArgs := []string{
		"-p", fmt.Sprintf("%d", p.Mesh.ProxyListenPort),
		"-u", strconv.FormatInt(p.SidecarProxyUID, 10),
	}
	if p.IncludeIPRanges != "" {
		initArgs = append(initArgs, "-i", p.IncludeIPRanges)
	}

	var pullPolicy v1.PullPolicy
	switch p.ImagePullPolicy {
	case "Always":
		pullPolicy = v1.PullAlways
	case "IfNotPresent":
		pullPolicy = v1.PullIfNotPresent
	case "Never":
		pullPolicy = v1.PullNever
	default:
		pullPolicy = v1.PullIfNotPresent
	}

	initContainer := v1.Container{
		Name:            InitContainerName,
		Image:           p.InitImage,
		Args:            initArgs,
		ImagePullPolicy: pullPolicy,
		SecurityContext: &v1.SecurityContext{
			Capabilities: &v1.Capabilities{
				Add: []v1.Capability{"NET_ADMIN"},
			},
		},
	}

	spec.InitContainers = append(spec.InitContainers, initContainer)

	// sidecar proxy container
	args := []string{"-v", "10", "--logtostderr"}

	volumeMounts := []v1.VolumeMount{
		{
			Name:      istioEnvoyConfigVolumeName,
			MountPath: p.Mesh.DefaultConfig.ConfigPath,
		},
	}

	spec.Volumes = append(spec.Volumes,
		v1.Volume{
			Name: istioEnvoyConfigVolumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		})

	sidecar := v1.Container{
		Name:  ProxyContainerName,
		Image: "gcr.io/istio-testing/proxy2:latest",
		Args:  args,
		Env: []v1.EnvVar{{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		}, {
			Name: "POD_NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &p.SidecarProxyUID,
		},
		VolumeMounts: volumeMounts,
	}

	spec.Containers = append(spec.Containers, sidecar)
}

func intoObject(c *Config, in interface{}) (interface{}, error) {
	obj, err := meta.Accessor(in)
	if err != nil {
		return nil, err
	}

	out, err := injectScheme.DeepCopy(in)
	if err != nil {
		return nil, err
	}

	if !injectRequired(c.IncludeNamespaces, ignoredNamespaces, c.ExcludeNamespaces, c.Policy, obj) {
		glog.V(2).Infof("Skipping %s/%s due to policy check", obj.GetNamespace(), obj.GetName())
		return out, nil
	}

	// `in` is a pointer to an Object. Dereference it.
	outValue := reflect.ValueOf(out).Elem()

	var objectMeta *metav1.ObjectMeta
	var templateObjectMeta *metav1.ObjectMeta
	var templatePodSpec *v1.PodSpec
	// CronJobs have JobTemplates in them, instead of Templates, so we
	// special case them.
	if job, ok := out.(*v2alpha1.CronJob); ok {
		objectMeta = &job.ObjectMeta
		templateObjectMeta = &job.Spec.JobTemplate.ObjectMeta
		templatePodSpec = &job.Spec.JobTemplate.Spec.Template.Spec
	} else {
		templateValue := outValue.FieldByName("Spec").FieldByName("Template")
		// `Template` is defined as a pointer in some older API
		// definitions, e.g. ReplicationController
		if templateValue.Kind() == reflect.Ptr {
			templateValue = templateValue.Elem()
		}
		objectMeta = outValue.FieldByName("ObjectMeta").Addr().Interface().(*metav1.ObjectMeta)
		templateObjectMeta = templateValue.FieldByName("ObjectMeta").Addr().Interface().(*metav1.ObjectMeta)
		templatePodSpec = templateValue.FieldByName("Spec").Addr().Interface().(*v1.PodSpec)
	}

	// Skip injection when host networking is enabled. The problem is
	// that the iptable changes are assumed to be within the pod when,
	// in fact, they are changing the routing at the host level. This
	// often results in routing failures within a node which can
	// affect the network provider within the cluster causing
	// additional pod failures.
	if templatePodSpec.HostNetwork {
		return out, nil
	}

	for _, m := range []*metav1.ObjectMeta{objectMeta, templateObjectMeta} {
		if m.Annotations == nil {
			m.Annotations = make(map[string]string)
		}
		m.Annotations[istioSidecarAnnotationStatusKey] = "injected-version-" + c.Params.Version
	}

	injectIntoSpec(&c.Params, templatePodSpec, templateObjectMeta)

	return out, nil
}

// IntoResourceFile injects the istio proxy into the specified
// kubernetes YAML file.
func IntoResourceFile(c *Config, in io.Reader, out io.Writer) error {
	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		var typeMeta metav1.TypeMeta
		if err = yaml.Unmarshal(raw, &typeMeta); err != nil {
			return err
		}

		gvk := schema.FromAPIVersionAndKind(typeMeta.APIVersion, typeMeta.Kind)
		obj, err := injectScheme.New(gvk)
		var updated []byte
		if err == nil {
			if err = yaml.Unmarshal(raw, obj); err != nil {
				return err
			}
			out, err := intoObject(c, obj) // nolint: vetshadow
			if err != nil {
				return err
			}
			if updated, err = yaml.Marshal(out); err != nil {
				return err
			}
		} else {
			updated = raw // unchanged
		}
		if _, err = out.Write(updated); err != nil {
			return err
		}
		if _, err = fmt.Fprint(out, "---\n"); err != nil {
			return err
		}
	}
	return nil
}
*/
