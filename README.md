# tiny-compose





在environment   添加扩展控制变量 extra_hosts





## 1 网络模式为 bridge 且仅暴露一个端口，支持单机多实例部署





在创建伪redis 集群时可快速提供多个redis 实例



```
version: '3'
services:
  redis:
    image: redis:4
    container_name: redis
    network_mode: bridge
    restart: always
    volumes:
       - /data/redis-[outPort]/:/data
    environment:
       - TZ=Asia/Shanghai
       - extra_hosts="[172.16.100.3:7001-7003,172.16.100.4:7001-7003]"
       - extra_env="[volTag=outPort,testEnv1=xxx-host,testEnv2=xxx-outPort,testEnv3=xxx-host-outPort]"
    command: [
      "bash", "-c",
      '
       docker-entrypoint.sh
       --requirepass "redis-pwd"
       --appendonly yes
      '
    ]
    ports:
      - "6379:6379"
    logging:
      options:
        max-size: '100m'
        max-file: '10'
    extra_hosts:
      - "x.y.z:127.0.0.1"

```



同时创建6个redis 实例 对外暴露的端口分别为



```
172.16.100.3:7001 172.16.100.3:7002 172.16.100.3:7003
172.16.100.4:7001 172.16.100.4:7003 172.16.100.4:7003
```







```
./tiny-compose  -f example/redis-docker-compose.yaml 
```









## 1 网络模式为host 仅支持在一个服务器上部署一个实例



```
version: '3.2'
services:
  node-exporter:
    image: prom/node-exporter:latest
    container_name: "node-exporter"
    restart: unless-stopped    
    privileged: true
    network_mode: "host"
    environment:
      - extra_hosts="[172.16.100.3,172.16.100.4]"
    command:
      - '--path.procfs=/host/proc'      
      - '--path.rootfs=/rootfs'
      - '--path.sysfs=/host/sys'
      - '--collector.filesystem.ignored-mount-points=^/(sys|proc|dev|host|etc)($$|/)'
      - '--collector.textfile.directory=/node_exporter/prom'    
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro

```





```
172.16.100.3 172.16.100.4  各部署一个node-exporter实例
```



```
./tiny-compose  -f example/node-exporter.yml 
```





## 2 网络模式为bridge 暴露端口数量大于1 仅支持一个服务器部署一个实例





```
version: '3'
services:
  mysql:
    image: mysql:8.0.26
    network_mode: bridge
    privileged: true
    container_name: mysql
    restart: always
    volumes:
      - /data/mysql/data:/var/lib/mysql
      - /data/mysql/log:/var/log:rw
    ports:
      - 3306:3306
      - 33060:33060
    environment:
      - TZ=Asia/Shanghai
      - MYSQL_ROOT_PASSWORD=root@123
      - extra_hosts="[172.16.100.3,172.16.100.4]"

    command: 
     --default-authentication-plugin=mysql_native_password 
     --lower-case-table-names=1
     --character-set-server=utf8mb4
     --collation-server=utf8mb4_general_ci
     --max_allowed_packet=128M;
     --explicit_defaults_for_timestamp=true
     --max_connections=1500
     --skip-name-resolve=1
     --group_concat_max_len=102400
    logging:
      options:
        max-size: "100m"
        max-file: "10"
```



```
172.16.100.3 172.16.100.4  各部署一个mysql实例
```



```
./tiny-compose  -f example/mysql-docker-compose.yaml
```









