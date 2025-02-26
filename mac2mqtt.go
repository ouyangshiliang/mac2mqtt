package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"bytes"


	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var hostname string
var topic string

type config struct {
	Ip       string `yaml:"mqtt_ip"`
	Port     string `yaml:"mqtt_port"`
	User     string `yaml:"mqtt_user"`
	Password string `yaml:"mqtt_password"`
	Topic    string `yaml:"mqtt_topic"`
	Hostname string `yaml:"hostname"`
}

func (c *config) getConfig() *config {

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	configContent, err := os.ReadFile(exPath + "/mac2mqtt.yaml")
	if err != nil {
		log.Fatal("No config file provided")
	}

	err = yaml.Unmarshal(configContent, c)
	if err != nil {
		log.Fatal("No data in config file")
	}

	if c.Ip == "" {
		log.Fatal("Must specify mqtt_ip in mac2mqtt.yaml")
	}

	if c.Port == "" {
		log.Fatal("Must specify mqtt_port in mac2mqtt.yaml")
	}

	if c.User == "" {
		log.Fatal("Must specify mqtt_user in mac2mqtt.yaml")
	}

	if c.Password == "" {
		log.Fatal("Must specify mqtt_password in mac2mqtt.yaml")
	}

	if c.Topic == "" {
		log.Fatal("Must specify mqtt_topic in mac2mqtt.yaml")
	}

	if c.Hostname == "" {
		log.Fatal("must specify a hostname in mac2mqtt.yaml")
	}

	return c
}

func getHostname() string {

	hostname, err := os.Hostname()

	if err != nil {
		log.Fatal(err)
	}

	// "name.local" => "name"
	firstPart := strings.Split(hostname, ".")[0]

	// remove all symbols, but [a-zA-Z0-9_-]
	reg, err := regexp.Compile("[^a-zA-Z0-9_-]+")
	if err != nil {
		log.Fatal(err)
	}
	firstPart = reg.ReplaceAllString(firstPart, "")

	return firstPart
}

func getCommandOutput(name string, arg ...string) string {
	cmd := exec.Command(name, arg...)
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
		log.Println("Cmd err:")
	}

	stdoutStr := string(stdout)
	stdoutStr = strings.TrimSuffix(stdoutStr, "\n")

	return stdoutStr
}

func getMuteStatus() bool {
	output := getCommandOutput("/usr/bin/osascript", "-e", "output muted of (get volume settings)")

	b, err := strconv.ParseBool(output)
	if err != nil {
		log.Fatal(err)
	}

	return b
}

func getCurrentVolume() int {
	output := getCommandOutput("/usr/bin/osascript", "-e", "output volume of (get volume settings)")

	i, err := strconv.Atoi(output)
	if err != nil {
		log.Fatal(err)
	}

	return i
}

func getCurrentLock() bool {
	    // 构建要执行的命令字符串
    cmdStr := `/usr/libexec/PlistBuddy -c "print :IOConsoleUsers:0:CGSSessionScreenIsLocked" /dev/stdin 2>/dev/null <<< "$(ioreg -n Root -d1 -a)"`
    // 使用/bin/sh -c来执行包含shell特性的命令
    cmd := exec.Command("/bin/sh", "-c", cmdStr)
    // 捕获命令的输出
    var out bytes.Buffer
    cmd.Stdout = &out
    // 忽略命令的错误输出
    cmd.Stderr = nil
    // 运行命令
    err := cmd.Run()
    if err != nil {
        return false
    }
	  result := strings.TrimSpace(out.String())

	// 将字符串转换为布尔值
	l, err := strconv.ParseBool(result)
	if err != nil {
		return false // 如果转换失败，返回错误
	}
	  	return l // 返回屏幕锁定状态和nil错误（表示成功）
}

func runCommand(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
/*    //start
    var cmdStdout bytes.Buffer
    var cmdStderr bytes.Buffer
    cmd.Stdout = &cmdStdout
    cmd.Stderr = &cmdStderr
    log.Println("Stdout:"  + cmdStdout.String()  + ", Stderr from cmd:" + cmdStderr.String())
    //end*/
	_, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
}

// from 0 to 100
func setVolume(i int) {
	runCommand("/usr/bin/osascript", "-e", "set volume output volume "+strconv.Itoa(i))
}

// true - turn mute on
// false - turn mute off
func setMute(b bool) {
	runCommand("/usr/bin/osascript", "-e", "set volume output muted "+strconv.FormatBool(b))
}

func setLock() {
	runCommand("pmset", "displaysleepnow")
}

func setUnlock() {
	runCommand("/usr/bin/caffeinate", "-u", "-t", "1")
}

func commandSleep() {
	runCommand("pmset", "sleepnow")
}

func commandDisplaySleep() {
	runCommand("pmset", "displaysleepnow")
}

func commandDisplayWake() {
	runCommand("/usr/bin/caffeinate", "-u", "-t", "1")
}

func commandRunShortcut(shortcut string) {
	runCommand("shortcuts", "run", shortcut)
}

func commandScreensaver() {
	runCommand("open", "-a", "ScreenSaverEngine")
}

func commandShutdown() {

	if os.Getuid() == 0 {
		// if the program is run by root user we are doing the most powerfull shutdown - that always shuts down the computer
		runCommand("shutdown", "-h", "now")
	} else {
		// if the program is run by ordinary user we are trying to shutdown, but it may fail if the other user is logged in
		runCommand("/usr/bin/osascript", "-e", "tell app \"System Events\" to shut down")
	}

}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Println("Connected to MQTT")

	token := client.Publish(getTopicPrefix()+"/status/alive", 0, true, "true")
	token.Wait()

	log.Println("Sending 'true' to topic: " + getTopicPrefix() + "/status/alive")

	listen(client, getTopicPrefix()+"/command/#")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Printf("Disconnected from MQTT: %v", err)
}

func getMQTTClient(ip, port, user, password string) mqtt.Client {

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", ip, port))
	opts.SetUsername(user)
	opts.SetPassword(password)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	opts.SetWill(getTopicPrefix()+"/status/alive", "false", 0, true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return client
}

func getTopicPrefix() string {
	return topic
}

func listen(client mqtt.Client, topic string) {

	token := client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {

		if msg.Topic() == getTopicPrefix()+"/command/volume" {

			i, err := strconv.Atoi(string(msg.Payload()))
			if err == nil && i >= 0 && i <= 100 {

				setVolume(i)

				updateVolume(client)
				updateMute(client)

			} else {
				log.Println("Incorrect value")
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/mute" {

			b, err := strconv.ParseBool(string(msg.Payload()))
			if err == nil {
				setMute(b)

				updateVolume(client)
				updateMute(client)

			} else {
				log.Println("Incorrect value")
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/lock" {

			//l, err := strconv.ParseBool(string(msg.Payload()))
			if string(msg.Payload()) == "lock" {
				setLock()
				updateVolume(client)
				updateMute(client)
				updateLock(client)

			} else {
				setUnlock()
				//log.Println("Incorrect value")
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/sleep" {

			if string(msg.Payload()) == "sleep" {
				commandSleep()
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/displaysleep" {

			if string(msg.Payload()) == "displaysleep" {
				commandDisplaySleep()
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/shutdown" {

			if string(msg.Payload()) == "shutdown" {
				commandShutdown()
			}

		}

	})

	token.Wait()
	if token.Error() != nil {
		log.Printf("Token error: %s\n", token.Error())
	}
}

func updateVolume(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/volume", 0, false, strconv.Itoa(getCurrentVolume()))
	token.Wait()
}

func updateMute(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/mute", 0, false, strconv.FormatBool(getMuteStatus()))
	token.Wait()
}

func updateLock(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/lock", 0, false, strconv.FormatBool(getCurrentLock()))
	token.Wait()
}

func getBatteryChargePercent() string {

	output := getCommandOutput("/usr/bin/pmset", "-g", "batt")

	// $ /usr/bin/pmset -g batt
	// Now drawing from 'Battery Power'
	//  -InternalBattery-0 (id=4653155)        100%; discharging; 20:00 remaining present: true

	r := regexp.MustCompile(`(\d+)%`)
	res := r.FindStringSubmatch(output)
	if len(res) == 0 {
		return ""
	}

	return res[1]
}

func updateBattery(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/battery", 0, false, getBatteryChargePercent())
	token.Wait()
}

func main() {

	log.Println("Started")

	var c config
	c.getConfig()

	if c.Topic == "" {
		topic = "mac2mqtt/" + hostname
	} else {
		topic = c.Topic
	}

	hostname = c.Hostname

	var wg sync.WaitGroup

	//hostname = getHostname()
	mqttClient := getMQTTClient(c.Ip, c.Port, c.User, c.Password)

	volumeTicker := time.NewTicker(2 * time.Second)
	batteryTicker := time.NewTicker(60 * time.Second)

	wg.Add(1)
	go func() {
		for {
			select {
			case _ = <-volumeTicker.C:
				updateVolume(mqttClient)
				updateMute(mqttClient)
				updateLock(mqttClient)

			case _ = <-batteryTicker.C:
				updateBattery(mqttClient)
			}
		}
	}()

	wg.Wait()

}
