# Injection script for inserting sidecar and iptables containers.
# Modelled after istio kube-inject.
# The input is a kubernetes resource JSON.
function(o, image="gcr.io/istio-testing/envoymesh:latest")
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
                        name: "istio-proxy",
                        securityContext: {
                            runAsUser: 1337,
                        },

                    }],
                    initContainers+: [{
                        args: ["-p", "15001", "-u", "1337"],
                        image: "docker.io/istio/proxy_init:0.4.0",
                        name: "istio-init",
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
