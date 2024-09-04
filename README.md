# 基于vnstat Api实现的流量监控

注意：网卡名称参数`INTERFACE`一定要指定否则不知道监控哪个网卡，每个服务器网卡名称都不一样，可以用`ip a`查看网卡

---

挂载dbus关机方式 + 企业微信通知 + 邮件通知
```
docker run -d --name network-monitor --net=host \
    -v /proc/uptime:/proc/uptime \
    -v /var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket \
    -e NAME=gcp \
    -e HOST=127.0.0.1:28685 \
    -e GB=180 \
    -e INTERFACE=ens4 \
    -e WXKEY=xxxxxxxxxxxx \
    -e SHUTDOWN=yes \
    -e SMTPEMAIL=xxxxxx@qq.com \
    -e SMTPPWD=xxxxxx \
    javaow/network-monitor
```
ssh关机方式 + 企业微信通知
```
docker run -d --name network-monitor --net=host \
    -v /proc/uptime:/proc/uptime \
    -e NAME=gcp \
    -e HOST=127.0.0.1:28685 \
    -e GB=180 \
    -e INTERFACE=ens4 \
    -e WXKEY=xxxxxxxxxxxx \
    -e SHUTDOWN=yes \
    -e SHUTDOWNTYPE=ssh \
    -e SSHHOST=root@127.0.0.1 \
    -e SSHPWD=xxxxxx \
    javaow/network-monitor
```


## 镜像-e 参数

|选项|解释|
|---|---|
|NAME|自定义名称 用于通知时分辨机器|
|HOST|vnstat的IP和端口 格式：IP:Port|
|GB|限额流量 单位GB 可小数 流量消耗到该参数时将触发通知和关机操作|
|MODEL|模式 默认为1（1:以上行流量为限制 2:上下行合并后限制 用于上下行都计算流量的vps）|
|INTERFACE|网卡名称 默认eth0 必填|
|INTERVAL|监听间隔 单位：秒 默认30秒（vnstat是5分钟记录一次 所以该参数默认即可）|
|PARDON|开机延迟时间 单位：秒 默认10分钟（开机后前10分钟不监听 避免开机就关机造成死循环）|
|SHUTDOWN|超额后是否关机 默认no|
|SHUTDOWNTYPE|关机方式 二进制使用host 容器使用ssh和dbus 默认dbus|
|SSHHOST|【ssh关机】ssh用户名和host 格式为：xxx@xx.xx.xx.xx|
|SSHPWD|【ssh关机】ssh密码|
|SSHPORT|【ssh关机】ssh端口 默认22|
|WXKEY|【微信通知】企业微信WebHook的key|
|SMTPHOST|【邮件通知】smtp服务器 默认为qq smtp.qq.com:587|
|SMTPEMAIL|【邮件通知】smtp发送邮箱和接收邮箱 发送给自己|
|SMTPPWD|【邮件通知】smtp密码|
