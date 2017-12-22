# Injection script for inserting sidecar and iptables containers.
# Modelled after istio kube-inject.
# The input is a kubernetes resource JSON.
function(o,
         image="gcr.io/istio-testing/envoysidecar:latest",
         uid=1337,
         port=15001)
    if o.kind == 'Deployment' then o {
        spec: super.spec + {
            template: super.template + {
                spec: super.spec {
                    containers+: [{
                        args: ["--id", "$(POD_NAMESPACE)/$(POD_NAME)", "--ads", "envoycontroller"],
                        env: [
                            {
                                name: "POD_NAME",
                                valueFrom: {
                                    fieldRef: {
                                        fieldPath: "metadata.name",
                                    },
                                },
                            },
                            {
                                name: "POD_NAMESPACE",
                                valueFrom: {
                                    fieldRef: {
                                        fieldPath: "metadata.namespace",
                                    },
                                },
                            },
                        ],
                        image: image,
                        name: "envoy",
                        securityContext: {
                            runAsUser: uid,
                        },
                        volumeMounts: [{
                            mountPath: "/tmp",
                            name: "envoy-config",
                        }],
                    }],
                    initContainers+: [{
                        args: ["-p", std.toString(port), "-u", std.toString(uid)],
                        image: "docker.io/istio/proxy_init:0.4.0",
                        name: "iptables",
                        securityContext: {
                            capabilities: {
                                add: ["NET_ADMIN"],
                            },
                        },
                    }],
                    volumes+: [{
                        name: "envoy-config",
                        emptyDir: { medium: "Memory" },
                    }],
                },
            },
        },
    } else o
