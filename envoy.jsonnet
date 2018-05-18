local util = {
    longest_suffix(a, b, j)::
        if j >= std.length(a) || j >= std.length(b) then
            j
        else if a[std.length(a) - 1 - j] != b[std.length(b) - 1 - j] then
            j
        else
            self.longest_suffix(a, b, j + 1),

    domains(service, port, domain)::
        local service_names = std.split(service.hostname, '.');
        local context_names = std.split(domain, '.');
        local j = self.longest_suffix(service_names, context_names, 0);
        local expansions = [
            std.join('.', service_names[0:std.length(service_names) - i])
            for i in std.range(0, j)
        ] + if 'address' in service then [service.address] else [];
        expansions + ['%s:%d' % [host, port] for host in expansions],

};

local model = {
    key(hostname, labels, port_desc)::
        local labels_strings = ['%s=%s' % [key, labels[key]] for key in std.objectFields(labels)];
        '%s|%s|%s' % [hostname, port_desc.name, std.join(',', std.sort(labels_strings))],

    is_http2(protocol)::
        protocol == 'HTTP2' || protocol == 'GRPC',

    is_http(protocol)::
        protocol == 'HTTP' || self.is_http2(protocol),

    is_tcp(protocol)::
        protocol == 'TCP' || protocol == 'HTTPS',

    is_udp(protocol)::
        protocol == 'UDP',
};

local config = {
    inbound_cluster(port, protocol)::
        {
            name: 'in.%d' % [port],
            connect_timeout: '5s',
            type: 'STATIC',
            lb_policy: 'ROUND_ROBIN',
            hosts: [{
                socket_address: {
                    address: '127.0.0.1',
                    port_value: port,
                },
            }],
            [if model.is_http2(protocol) then 'http2_protocol_options']: {},
        },

    outbound_cluster(hostname, labels, port_desc)::
        local key = model.key(hostname, labels, port_desc);
        {
            name: key,
            connect_timeout: '5s',
            type: 'EDS',
            eds_cluster_config: {
                service_name: key,
                eds_config: { ads: {} },
            },
            lb_policy: 'ROUND_ROBIN',
            hostname:: hostname,
            [if model.is_http2(port_desc.protocol) then 'http2_protocol_options']: {},
        },

    default_route(cluster, operation)::
        {
            match: {
                prefix: '/',
            },
            route: {
                cluster: cluster.name,
            },
            decorator: {
                operation: operation,
            },
        },

    inbound_listeners(uid, instances)::
        [{
            local protocol = instance.endpoint.service_port.protocol,
            local port = instance.endpoint.port,
            local cluster = config.inbound_cluster(port, protocol),
            local prefix = 'in_%s_%d' % [protocol, port],
            name: 'in_%s_%s_%d' % [protocol, instance.endpoint.ip_address, port],
            cluster:: cluster,
            address: {
                socket_address: {
                    address: instance.endpoint.ip_address,
                    port_value: port,
                },
            },
            filter_chains: [
                {
                    filters: [
                        if model.is_http(protocol) then
                            {
                                name: 'envoy.http_connection_manager',
                                config: {
                                    stat_prefix: prefix,
                                    codec_type: 'AUTO',
                                    access_log: [{
                                        name: 'envoy.file_access_log',
                                        config: { path: '/dev/stdout' },
                                    }],
                                    generate_request_id: true,
                                    route_config: {
                                        name: prefix,
                                        virtual_hosts: [{
                                            name: prefix,
                                            domains: ['*'],
                                            routes: [config.default_route(cluster, 'inbound_route')],
                                        }],
                                        validate_clusters: false,
                                    },
                                    http_filters: [{
                                        name: 'mixer',
                                        config: {
                                            default_destination_service: instance.service.hostname,
                                            service_configs: {
                                                [instance.service.hostname]: {
                                                    disable_check_calls: true,
                                                    mixer_attributes: {
                                                        attributes: {
                                                            'destination.service': { string_value: 'ingress' },  // Access from outside the mesh
                                                            'destination.uid': { string_value: uid },
                                                            'context.reporter.proxy': { string_value: 'server' },
                                                            'context.reporter.id': { string_value: uid },
                                                        },
                                                    },
                                                },
                                            },
                                            transport: {
                                                check_cluster: model.key('istio-policy.istio-system.svc.cluster.local', {}, { name: 'grpc-mixer' }),
                                                report_cluster: model.key('istio-telemetry.istio-system.svc.cluster.local', {}, { name: 'grpc-mixer' }),
                                            },
                                        },
                                    }, {
                                        name: 'envoy.router',
                                    }],
                                },
                            }
                        else if !model.is_udp(protocol) then
                            {
                                name: 'envoy.tcp_proxy',
                                config: {
                                    stat_prefix: prefix,
                                    cluster: cluster.name,
                                },
                            },
                    ],
                },
            ],
        } for instance in instances],

    outbound_http_ports(services)::
        std.set([
            port.port
            for service in services
            for port in service.ports
            if model.is_http(port.protocol)
        ]),

    outbound_http_routes(services, port, domain)::
        {
            name: '%d' % [port],
            virtual_hosts: [
                {
                    local cluster = config.outbound_cluster(service.hostname, {}, port_desc),
                    name: '%s:%d' % [service.hostname, port_desc.port],
                    cluster:: cluster,
                    domains: util.domains(service, port_desc.port, domain),
                    routes: [
                        {
                            match: {
                                prefix: '/',
                            },
                            route: {
                                cluster: cluster.name,
                            },
                            decorator: {
                                operation: 'default_route',
                            },
                            per_filter_config: {
                                mixer: {
                                    disable_check_calls: true,
                                    mixer_attributes: {
                                        attributes: {
                                            'destination.service': { string_value: service.hostname },
                                        },
                                    },
                                    forward_attributes: {
                                        attributes: {
                                            'destination.service': { string_value: service.hostname },
                                        },
                                    },
                                },
                                [if service.hostname == 'ratings.default.svc.cluster.local' then 'envoy.fault']: {
                                    abort: { http_status: 500, percent: 100 },
                                },
                            },
                        },
                    ],
                }
                for service in services
                for port_desc in service.ports
                if model.is_http(port_desc.protocol) && port_desc.port == port
            ],
            validate_clusters: false,
        },

    outbound_listeners(uid, services)::
        [
            {
                local prefix = 'out_%s_%s_%d' % [port.protocol, service.hostname, port.port],
                local cluster = config.outbound_cluster(service.hostname, {}, port),
                name: prefix,
                cluster:: cluster,
                address: {
                    socket_address: {
                        address: service.address,
                        port_value: port.port,
                    },
                },
                filter_chains: [
                    {
                        filters: [
                            {
                                name: 'envoy.tcp_proxy',
                                config: {
                                    stat_prefix: prefix,
                                    cluster: cluster.name,
                                },
                            },
                        ],
                    },
                ],
            }
            for service in services
            if 'address' in service
            for port in service.ports
            if model.is_tcp(port.protocol)
        ] + [
            {
                local prefix = 'out_HTTP_%d' % [port],
                name: prefix,
                address: {
                    socket_address: {
                        address: '0.0.0.0',
                        port_value: port,
                    },
                },
                filter_chains: [
                    {
                        filters: [
                            {
                                name: 'envoy.http_connection_manager',
                                config: {
                                    stat_prefix: prefix,
                                    codec_type: 'AUTO',
                                    access_log: [{
                                        name: 'envoy.file_access_log',
                                        config: { path: '/dev/stdout' },
                                    }],
                                    generate_request_id: true,
                                    rds: {
                                        config_source: { ads: {} },
                                        route_config_name: '%d' % [port],
                                    },
                                    http_filters: [
                                        {
                                            name: 'mixer',
                                            config: {
                                                mixer_attributes: {
                                                    attributes: {
                                                        'source.uid': { string_value: uid },
                                                        'context.reporter.proxy': { string_value: 'client' },
                                                        'context.reporter.id': { string_value: uid },
                                                    },
                                                },
                                                forward_attributes: {
                                                    attributes: {
                                                        'source.uid': { string_value: uid },
                                                    },
                                                },
                                                transport: {
                                                    check_cluster: model.key('istio-policy.istio-system.svc.cluster.local', {}, { name: 'grpc-mixer' }),
                                                    report_cluster: model.key('istio-telemetry.istio-system.svc.cluster.local', {}, { name: 'grpc-mixer' }),
                                                },
                                            },
                                        },
                                        {
                                            name: 'envoy.fault',
                                        },
                                        {
                                            name: 'envoy.router',
                                        },
                                    ],
                                },
                            },
                        ],
                    },
                ],
            }
            for port in config.outbound_http_ports(services)
        ],

    virtual_listener(port)::
        {
            name: 'virtual',
            address: {
                socket_address: {
                    address: '0.0.0.0',
                    port_value: port,
                },
            },
            use_original_dst: true,
            filter_chains: [{ filters: [] }],
        },

    sidecar_listeners(uid, instances, services)::
        [
            listener { deprecated_v1+: { bind_to_port: false } }
            for listener in config.inbound_listeners(uid, instances) + config.outbound_listeners(uid, services)
        ],
};

function(services=import 'testdata/services.json',
         instances=import 'testdata/instances.json',
         uid='kubernetes://pod1.ns2',
         domain='default.svc.cluster.local',
         port=15001)
    {
        listeners: [config.virtual_listener(port)] +
                   config.sidecar_listeners(uid, instances, services),
        routes: [
            config.outbound_http_routes(services, port, domain)
            for port in config.outbound_http_ports(services)
        ],
        clusters: [
            listener.cluster
            for listener in self.listeners
            if 'cluster' in listener
        ] + [
            host.cluster
            for route in self.routes
            for host in route.virtual_hosts
        ],
    }
