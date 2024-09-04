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
	"net/smtp"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

func GetLastestBootTime() (int64, error) {
	// 获取开机时间
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

func SendWx(text string) {

	param := strings.NewReader(`{"msgtype":"text","text":{"content":"` + text + `"}}`)
	req, _ := http.NewRequest("POST", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key="+*wxKey, param)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("发送到企业微信错误: %v", err)
	}
	log.Println("企业微信发送成功!")
	defer resp.Body.Close()
}

func SendEmail(body string) {

	smtpServer, smtpPort, err := net.SplitHostPort(*smtpHost)
	if err != nil {
		log.Println("邮件服务器解析错误:", err)
		return
	}
	subject := "服务器流量耗尽通知"

	// 邮件内容格式
	message := "From: " + *smtpEmail + "\n" +
		"To: " + *smtpEmail + "\n" +
		"Subject: " + subject + "\n\n" +
		body

	auth := smtp.PlainAuth("", *smtpEmail, *smtpPwd, smtpServer)

	// 连接邮件服务器
	err = smtp.SendMail(smtpServer+":"+smtpPort, auth, *smtpEmail, []string{*smtpEmail}, []byte(message))
	if err != nil {
		log.Println("邮件发送报错:", err)
		return
	}
	log.Println("通知邮件发送成功!")
}

func PrintLog(msg string) {
	log.Println(msg)
	if *wxKey != "" {
		SendWx(msg)
	}
	if *smtpEmail != "" && *smtpHost != "" {
		SendEmail(msg)
	}
}

func ConvertFileSize(size float64) float64 {
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
			log.Println("反序列化 JSON 时出错:", err)
			return jsonData, err
		}

		return jsonData, nil
	} else {
		return jsonData, errors.New("返回状态码不及预期")
	}
}

func Exceed(nodeName, interfaceName string, tx, rx float64) {
	msg := fmt.Sprintf("【%s】流量超额：%s：↑ %.2fGB  ↓ %.2fGB", nodeName, interfaceName, tx, rx)
	PrintLog(msg)
	errMsg := ""

	if *shutdown == "yes" {
		log.Println("执行关机...")

		if *shutdownType == "host" {
			cmd := exec.Command("shutdown", "-h", "now")
			e := cmd.Run()
			if e != nil {
				errMsg = fmt.Sprintf("【%s】shutdown关机命令执行失败：%s", *name, e.Error())
				PrintLog(errMsg)
			}
		}

		if *shutdownType == "dbus" {
			cmd := exec.Command("dbus-send", "--system", "--print-reply", "--dest=org.freedesktop.login1", "/org/freedesktop/login1", "org.freedesktop.login1.Manager.PowerOff", "boolean:true")
			e := cmd.Run()
			if e != nil {
				errMsg = fmt.Sprintf("【%s】dbus关机命令执行失败：%s", *name, e.Error())
				PrintLog(errMsg)
			}
		}

		if *shutdownType == "ssh" {
			cmd := exec.Command("/usr/bin/sshpass", "-p", *sshPwd, "/usr/bin/ssh", "-o", "StrictHostKeyChecking=no", *sshHost, "-p", *sshPort, "shutdown -h now")
			e := cmd.Run()
			if e != nil {
				if e.Error() != "exit status 255" {
					errMsg = fmt.Sprintf("【%s】ssh关机命令执行失败：%s", *name, e.Error())
					PrintLog(errMsg)
				}
			}
		}
	}
}

func Task(url string) {

	startTime, err := GetLastestBootTime()
	if err != nil {
		errMsg := fmt.Sprintf("【%s】获取开机时间报错：%s", *name, err.Error())
		PrintLog(errMsg)
		return
	}

	if startTime < *start {
		log.Println("处于开机延迟时间 不监测")
		return
	}

	json, err := GetUrl(url)
	if err != nil {
		log.Println("Http请求发生错误：", err.Error())
		return
	}

	t := time.Now()

	for _, k := range json.Interfaces {
		if *interfacesName == k.Name {
			for _, m := range k.Traffic.Month {
				if t.Year() == m.Date.Year && int(t.Month()) == m.Date.Month {
					tx := ConvertFileSize(float64(m.Tx)) // 上传
					rx := ConvertFileSize(float64(m.Rx)) // 下载

					log.Printf("%s：%s：↑ %.2fGB  ↓ %.2fGB\n", *name, k.Name, tx, rx)

					if *model == 1 {
						if *gb < tx {
							// 上传流量达到限制
							Exceed(*name, k.Name, tx, rx)
						}
					} else if *model == 2 {
						if *gb < (tx + rx) {
							// 上传+下载流量达到限制
							Exceed(*name, k.Name, tx, rx)
						}
					}
				}
			}
		}
	}
}

func Verify() bool {
	flag.Parse()

	_, err := net.ResolveTCPAddr("tcp", *host)
	if err != nil {
		errMsg := fmt.Sprintf("【%s】vnstat的Host参数解析错误", *name)
		PrintLog(errMsg)
		return false
	}

	if *gb == 0 {
		errMsg := fmt.Sprintf("【%s】GB参数不能为0", *name)
		PrintLog(errMsg)
		return false
	}

	if *model != 1 && *model != 2 {
		errMsg := fmt.Sprintf("【%s】不存在的模式", *name)
		PrintLog(errMsg)
		return false
	}

	if *interfacesName == "" {
		errMsg := fmt.Sprintf("【%s】INTERFACE参数不能为空！", *name)
		PrintLog(errMsg)
		return false
	}

	if *shutdownType == "ssh" {
		if *sshHost == "" {
			errMsg := fmt.Sprintf("【%s】SSHHOST参数不能为空！", *name)
			PrintLog(errMsg)
			return false
		}
	}
	return true
}

var host = flag.String("host", "", "vnstat的IP和端口 格式：IP:Port")
var second = flag.Int64("interval", 30, "监听间隔 单位：秒 默认30")
var start = flag.Int64("pardon", (10 * 60), "开机延迟时间 单位：秒 默认10分钟")
var model = flag.Int64("model", 1, "模式 1:以上行流量为限制 2:上下行合并后为限制")
var gb = flag.Float64("gb", 1000000, "限额流量大小 单位：GB")
var interfacesName = flag.String("interface", "", "网卡名称 例：eth0")
var name = flag.String("name", "", "自定义名称 通知时使用")
var wxKey = flag.String("wxKey", "", "企业微信WebHook的key")
var shutdown = flag.String("shutdown", "no", "超额后是否关机")
var shutdownType = flag.String("shutdownType", "host", "关机方式 二进制使用host 容器使用ssh和dbus")
var sshHost = flag.String("sshHost", "", "ssh用户名和host 格式为：xxx@xx.xx.xx.xx")
var sshPwd = flag.String("sshPwd", "", "ssh密码")
var sshPort = flag.String("sshPort", "22", "ssh端口 默认22")
var smtpHost = flag.String("smtpHost", "smtp.qq.com:587", "smtp服务器 默认为qq smtp.qq.com:587")
var smtpEmail = flag.String("smtpEmail", "", "smtp发送邮箱和接收邮箱 发送给自己")
var smtpPwd = flag.String("smtpPwd", "", "smtp密码")

func main() {

	if !Verify() {
		return
	}

	url := "http://" + *host

	ticker := time.NewTicker(time.Duration(*second) * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				go Task(url)
			}
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for {
		select {
		case <-c:
			return
		}
	}
}
