# Injection script for inserting sidecar and iptables containers.
# Modelled after istio kube-inject.
# The input is a kubernetes resource JSON.
function(o, image="gcr.io/istio-testing/envoymesh:latest", uid=1337, port=15001)
    if o.kind == 'Deployment' then o {
        spec: super.spec + {
            template: super.template + {
                spec: super.spec {
                    containers+: [{
                        args: ["-v", "2", "--logtostderr"],
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
                        name: "sidecar",
                        securityContext: {
                            runAsUser: uid,
                        },

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
                },
            },
        },
    } else o
