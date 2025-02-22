# Mac2MQTT

`mac2mqtt` is a program that allows viewing and controlling some aspects of computers running macOS via MQTT.

This repo is a fork of bessarabov/mac2mqtt, that fixes bugs and adds some possibilities for the config.yaml (in example login without password).
Furthermore. The commands topic for displaywake or sleep is conform to the home assistant mqtt topic with /set, see below.

---

It publishes to MQTT:

 * current volume
 * volume mute state
 * battery charge percent
 * system lock status


You can send topics to:

 * change volume
 * mute/unmute
 * put the computer to sleep
 * shutdown computer
 * turn off the display
 * wake display
 * run macOS shortcuts

## Running

To run this program you need to put 2 files in a directory (`/Users/USERNAME/mac2mqtt/`):

    mac2mqtt
    mac2mqtt.yaml

Edit `mac2mqtt.yaml` (the sample file is in this repository), make binary executable (`chmod +x mac2mqtt`) and run `./mac2mqtt`:

    $ ./mac2mqtt
    2021/04/12 10:37:28 Started
    2021/04/12 10:37:29 Connected to MQTT
    2021/04/12 10:37:29 Sending 'true' to topic: iot/macmini/status/alive

## Running in the background

You need `mac2mqtt.yaml` and `mac2mqtt` to be placed in the directory `/Users/USERNAME/mac2mqtt/`,
then you need edit the file `com.bessarabov.mac2mqtt.plist`
and replace `USERNAME` with your username. Then put the file in `/Library/LaunchAgents/`.


And run:

    launchctl load /Library/LaunchAgents/com.bessarabov.mac2mqtt.plist

(To stop you need to run `launchctl unload /Library/LaunchAgents/com.bessarabov.mac2mqtt.plist`)

## Home Assistant sample config

![](https://user-images.githubusercontent.com/47263/114361105-753c4200-9b7e-11eb-833c-c26a2b7d0e00.png)

`configuration.yaml`:

```yaml
script:
  macmini_sleep:
    icon: mdi:laptop
    sequence:
      - service: mqtt.publish
        data:
          topic: "iot/macmini/command/sleep"
          payload: "sleep"

  macmini_shutdown:
    icon: mdi:laptop
    sequence:
      - service: mqtt.publish
        data:
          topic: "iot/macmini/command/shutdown"
          payload: "shutdown"

  macmini_displaysleep:
    icon: mdi:laptop
    sequence:
      - service: mqtt.publish
        data:
          topic: "iot/macmini/command/displaysleep"
          payload: "displaysleep"

mqtt:
  sensor:
    - name: macmini_alive
      icon: mdi:laptop
      state_topic: "iot/macmini/status/alive"

    - name: "macmini_battery"
      icon: mdi:battery-high
      unit_of_measurement: "%"
      state_topic: "iot/macmini/status/battery"
```

## MQTT topics structure

The program is working with several MQTT topics. All topics are prefixed with mqtt_topic or `mac2mqtt` + `COMPUTER_NAME`.
For example, the topic with the current volume on my machine is `iot/macmini/status/volume`

`mac2mqtt` send info to the topics `iot/macmini/status/#` and listen for commands in topics
`ot/macmini/command/#`.

### PREFIX + `/status/alive`

There can be `true` or `false` in this topic. If `mac2mqtt` is connected to MQTT server there is `true`.
If `mac2mqtt` is disconnected from MQTT there is `false`. This is the standard MQTT thing called Last Will and Testament.

### PREFIX + `/status/volume`

The value ranges from 0 (inclusive) to 100 (inclusive)—the current volume of the computer.

The value of this topic is updated every 60 seconds.

### PREFIX + `/status/mute`

There can be `true` or `false` in this topic. `true` means that the computer volume is muted (no sound),
`false` means that it is not muted.

### PREFIX + `/status/battery`

The value ranges from 0 (inclusive) to 100 (inclusive) and represents the current level of the battery. Returns empty if there is no battery.

The value of this topic is updated every 60 seconds.

### PREFIX + `/command/volume`

You can send integer numbers from 0 (inclusive) to 100 (inclusive) to this topic. It will set the volume on the computer.

### PREFIX + `/command/mute`

You can send `true` or `false` to this topic. When you send `true` the computer is muted. When you send `false` the computer
is unmuted.

### PREFIX + `/command/lock`

You can send `true` or `false` to this topic. When you send `true` the computer is locked. When you send `false` the computer
is unlocked.

### PREFIX + `/command/runshortcut`

You can send the name of a shortcut to this topic. It will run this shortcut in the Shortcuts app.

### PREFIX + `/command/set`

You can send `screensaver` to this topic. It will turn start your screensaver. Sending some other value will do nothing.

You can send `displaywake` to this topic. It will turn on the display. Sending some other value will do nothing.

You can send  `sleep` to this topic, and it will put the computer to sleep. Sending other values will do nothing.

You can send `shutdown` to this topic. It will try to shut down the computer. The way it is done depends on the user who ran the program. If the program is run by `root` the computer will shut down, but if it is run by an ordinary user the computer will not shut down if there is another user who logged in. Sending some other value but `shutdown` will do nothing.

You can send `displaysleep` to this topic. It will turn off the display. Sending some other value will do nothing.

You can send `lock` to this topic. It will lock the system. Sending some other value will do nothing.


## Building

To build this program yourself, follow these steps:

1. Clone this repo
2. Make sure you have installed go, for example with `brew install go`
3. Install its dependencies with `go install`
4. Build with `go build mac2mqtt.go`

It outputs a file `mac2mqtt`. Make the binary executable (`chmod +x mac2mqtt`) and run `./mac2mqtt`.
