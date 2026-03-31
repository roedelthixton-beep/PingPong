[Unit]
Description=EigenFlux %i service
Requires=eigenflux-etcd.service
After=network-online.target eigenflux-etcd.service
Wants=network-online.target

[Service]
Type=simple
User={{RUN_USER}}
Group={{RUN_GROUP}}
WorkingDirectory={{PROJECT_ROOT}}
ExecStartPre=/usr/bin/test -x {{PROJECT_ROOT}}/build/%i
ExecStart={{PROJECT_ROOT}}/build/%i
Restart=always
RestartSec=5
TimeoutStopSec=30
KillSignal=SIGTERM
NoNewPrivileges=true
LimitNOFILE=65535
SyslogIdentifier=eigenflux-%i

[Install]
WantedBy=multi-user.target
