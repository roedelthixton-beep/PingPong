[Unit]
Description=EigenFlux etcd via Docker Compose
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory={{PROJECT_ROOT}}
ExecStart=/usr/bin/docker compose -f {{PROJECT_ROOT}}/docker-compose.cloud.yml up -d
ExecStop=/usr/bin/docker compose -f {{PROJECT_ROOT}}/docker-compose.cloud.yml down
TimeoutStartSec=0
TimeoutStopSec=120

[Install]
WantedBy=multi-user.target
