package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

func GetLastestBootTime() (int64, error) {
	// 获取开机时间 返回已经开机了多少秒
	uptimeFilePath := "/proc/uptime"
	now := time.Now().Unix()

	fileBytes, err := os.ReadFile(uptimeFilePath)
	if err != nil {
		return 0, fmt.Errorf("读取/proc/uptime出错")
	}
	uptimeFileStr := string(fileBytes)

	arr := strings.Fields(uptimeFileStr)
	uptimefloat, err := strconv.ParseFloat(arr[0], 64)
	if err != nil {
		return 0, fmt.Errorf("解析开机时间出错")
	}
	return time.Now().Unix() - (now - int64(uptimefloat)), nil
}

type JsonData struct {
	Interfaces []struct {
		Name    string `json:"name"`
		Traffic struct {
			Total struct {
				Rx int `json:"rx"`
				Tx int `json:"tx"`
			} `json:"total"`
			Day []struct {
				ID   int `json:"id"`
				Date struct {
					Year  int `json:"year"`
					Month int `json:"month"`
					Day   int `json:"day"`
				} `json:"date"`
				Timestamp int `json:"timestamp"`
				Rx        int `json:"rx"`
				Tx        int `json:"tx"`
			} `json:"day"`
			Month []struct {
				ID   int `json:"id"`
				Date struct {
					Year  int `json:"year"`
					Month int `json:"month"`
				} `json:"date"`
				Timestamp int `json:"timestamp"`
				Rx        int `json:"rx"`
				Tx        int `json:"tx"`
			} `json:"month"`
			Year []struct {
				ID   int `json:"id"`
				Date struct {
					Year int `json:"year"`
				} `json:"date"`
				Timestamp int `json:"timestamp"`
				Rx        int `json:"rx"`
				Tx        int `json:"tx"`
			} `json:"year"`
		} `json:"traffic"`
	} `json:"interfaces"`
}

func sendWx(text string) {
	param := strings.NewReader(`{"msgtype":"text","text":{"content":"` + text + `"}}`)
	req, _ := http.NewRequest("POST", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key="+*wxKey, param)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("发送到企业微信错误: %v", err)
	}
	defer resp.Body.Close()
}

func convertFileSize(size float64) float64 {
	return size / (1024.0 * 1024.0 * 1024.0)
}

func GetUrl(url string) (jsonData JsonData, e error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", url+"/json.cgi", nil)
	resp, err := client.Do(req)
	if err != nil {
		return jsonData, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return jsonData, err
		}

		err = json.Unmarshal(body, &jsonData)
		if err != nil {
			fmt.Println("反序列化 JSON 时出错:", err)
			return jsonData, err
		}

		return jsonData, nil
	} else {
		return jsonData, errors.New("返回状态码不及预期")
	}
}

func exceed(nodeName, interfaceName string, tx, rx float64) {
	// 发送企业微信通知
	if *wxKey != "" {
		sendWx(fmt.Sprintf("流量超额：%s：%s：↑ %.2fGB  ↓ %.2fGB", nodeName, interfaceName, tx, rx))
	}
	// 关机
	if *shutdown == "yes" {
		fmt.Println("执行关机...")

		if *shutdownType == "host" {
			cmd := exec.Command("shutdown", "-h", "now")
			e := cmd.Run()
			if e != nil {
				fmt.Printf("关机命令执行失败: %s\n", e.Error())
				sendWx(fmt.Sprintf("%s：关机命令执行失败: %s\n", *name, e.Error()))
			}
		}

		if *shutdownType == "dbus" {
			cmd := exec.Command("dbus-send", "--system", "--print-reply", "--dest=org.freedesktop.login1", "/org/freedesktop/login1", "org.freedesktop.login1.Manager.PowerOff", "boolean:true")
			e := cmd.Run()
			if e != nil {
				fmt.Printf("关机命令执行失败: %s\n", e.Error())
				sendWx(fmt.Sprintf("%s：关机命令执行失败: %s\n", *name, e.Error()))
			}
		}

		if *shutdownType == "ssh" {
			cmd := exec.Command("/usr/bin/sshpass", "-p", *sshPwd, "/usr/bin/ssh", "-o", "StrictHostKeyChecking=no", *sshHost, "-p", *sshPort, "shutdown -h now")
			e := cmd.Run()
			if e != nil {
				fmt.Printf("关机命令执行失败: %s\n", e.Error())
				sendWx(fmt.Sprintf("%s：关机命令执行失败: %s\n", *name, e.Error()))
			}
		}
	}
}

func task(url string) {

	startTime, err := GetLastestBootTime()
	if err != nil {
		fmt.Printf("获取开机时间报错：%s\n", err.Error())
		sendWx(fmt.Sprintf("%s：获取开机时间报错：%s", *name, err.Error()))
		return
	}

	if startTime < *start {
		fmt.Println("处于开机延迟时间 不监测")
		return
	}

	json, err := GetUrl(url)
	if err != nil {
		fmt.Println("Http请求发生错误：", err.Error())
		return
	}

	t := time.Now()

	for _, k := range json.Interfaces {
		if *interfacesName == k.Name {
			for _, m := range k.Traffic.Month {
				if t.Year() == m.Date.Year && int(t.Month()) == m.Date.Month {
					tx := convertFileSize(float64(m.Tx)) // 上传
					rx := convertFileSize(float64(m.Rx)) // 下载

					if *gb < tx {
						// 上传流量达到限制
						exceed(*name, k.Name, tx, rx)
					}
					fmt.Printf("%s：%s：↑ %.2fGB  ↓ %.2fGB\n", *name, k.Name, tx, rx)
				}
			}
		}
	}
}

func verify() bool {
	flag.Parse()

	_, err := net.ResolveTCPAddr("tcp", *host)
	if err != nil {
		fmt.Println("Host参数输入错误")
		return false
	}

	if *gb == 0 {
		fmt.Println("GB参数不能为0")
		return false
	}

	if *interfacesName == "" {
		fmt.Println("INTERFACE参数不能为空！")
		return false
	}

	if *shutdownType == "ssh" {
		if *sshHost == "" {
			fmt.Println("SSHHOST参数不能为空！")
			return false
		}
	}

	return true
}

var host = flag.String("host", "", "vnstat的IP和端口 格式：IP:Port")
var second = flag.Int64("interval", 30, "监听间隔 单位：秒 默认30")
var start = flag.Int64("pardon", (10 * 60), "开机延迟时间 单位：秒 默认10分钟")
var gb = flag.Float64("gb", 1000000, "限额上传流量大小 单位：GB")
var interfacesName = flag.String("interface", "", "网卡名称 例：ens4")
var name = flag.String("name", "", "自定义名称 通知时使用")
var wxKey = flag.String("wxKey", "", "企业微信WebHook的key")
var shutdown = flag.String("shutdown", "no", "超额后是否关机")
var shutdownType = flag.String("shutdownType", "host", "关机方式 二进制使用host 容器使用ssh和dbus")
var sshHost = flag.String("sshHost", "", "ssh用户名和host 格式为：xxx@xx.xx.xx.xx")
var sshPwd = flag.String("sshPwd", "", "ssh密码")
var sshPort = flag.String("sshPort", "22", "ssh端口")

func main() {

	if !verify() {
		return
	}

	url := "http://" + *host

	// 创建一个定时向ticker内部C通道发送消息的任务
	ticker := time.NewTicker(time.Duration(*second) * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C: // 读取通道数据 这里会定时执行
				go task(url)
			}
		}
	}()

	// 监听退出操作，执行退出逻辑
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt) // 监听到退出逻辑后会向通道c发送一个os.Signal
	for {
		select {
		case <-c: // 通道有数据就代表退出
			return
		}
	}
}