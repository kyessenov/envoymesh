function(ads_host="envoycontroller",
         ads_port=8080,
         ads_cluster="ads",
         id="unknown-id")
    {
        node: {
            id: id,
            cluster: "unknown-cluster",
        },
        dynamic_resources: {
            lds_config: { ads: {} },
            cds_config: { ads: {} },
            ads_config: {
                api_type: "GRPC",
                cluster_names: [ads_cluster],
            },
        },
        static_resources: {
            clusters: [{
                name: ads_cluster,
                connect_timeout: "5s",
                type: "LOGICAL_DNS",
                hosts: [{
                    socket_address: {
                        address: ads_host,
                        port_value: ads_port,
                    },
                }],
                lb_policy: "ROUND_ROBIN",
                http2_protocol_options: {},
            }],
        },
        admin: {
            access_log_path: "/dev/null",
            address: {
                socket_address: {
                    address: "127.0.0.1",
                    port_value: 15000,
                },
            },
        },
    }
