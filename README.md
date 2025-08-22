# PlayGo

![Demo image](doc/demo.png)
**PlayGo** is a simple streaming video player built with [Wails](https://wails.io/).

## Features
- Supports protocols:
  - RTSP / RTSPS
  - RTMP / RTMPS
  - HTTP-FLV / HTTPS-FLV
  - HTTP-TS / HTTPS-TS
  - HTTP-MP4 / HTTPS-MP4
  - HLS-TS / HLS-fMP4
  - SRT
- Supports codecs:
  - H264
  - AAC
- Cross-platform support (Windows, macOS, Linux)
- Simple and intuitive user interface

## Build
To build the application, make sure [Wails](https://wails.io/) is installed:
```bash
wails build
```

## Running on Linux
On Linux, you may need to install additional packages like `gstreamer1.0-plugins-bad`.

For example, on Debian/Ubuntu-based systems:
```bash
sudo apt-get install gstreamer1.0-plugins-bad
```