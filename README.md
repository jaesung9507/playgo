# PlayGo

![Demo image](doc/demo.png)
**PlayGo** is a simple streaming video player built with [Wails](https://wails.io/).

## Features
PlayGo supports a wide range of streaming protocols, local file formats, and online platforms.

### Supported Protocols
| Protocol | Video Codec | Audio Codec | Container |
|--|--|--|--|
| RTSP / RTSPS | H264 | AAC | - |
| RTMP / RTMPS | H264, H265 | AAC | FLV |
| HTTP-FLV / HTTPS-FLV | H264 | AAC | FLV |
| HTTP-TS / HTTPS-TS | H264 | AAC | TS |
| HTTP-MP4 / HTTPS-MP4 | H264 | AAC | MP4 |
| HLS / LL-HLS | H264, H265 | AAC | TS, fMP4 |
| SRT | H264, H265 | AAC | TS |

### Local File Playback
| Extension | Video Codec | Audio Codec |
|--|--|--|
| FLV | H264 | AAC |
| TS | H264 | AAC |
| MP4 | H264 | AAC |

### Supported Platforms
The following platforms are supported via direct URL input.
| Platform | Service | Example |
|--|--|--|
| YouTube | Video | https://www.youtube.com/watch?v={videoID} |
| YouTube | Live | https://www.youtube.com/live/{videoID} |
| YouTube | Shorts | https://www.youtube.com/shorts/{videoID} |
| YouTube | Kids | https://www.youtubekids.com/watch?v={videoID} |
| YouTube | Music | https://music.youtube.com/watch?v={videoID} |
| TikTok | Video | https://www.tiktok.com/@{uniqueID}/video/{videoID} |
| TikTok | Live | https://www.tiktok.com/@{uniqueID}/live |
| CIME | Live | https://ci.me/@{channelSlug}/live |
| CIME | VOD | https://ci.me/@{channelSlug}/vods/{vodID} |
| CIME | Clip | https://ci.me/clips/{clipID} |
| CHZZK | Live | https://chzzk.naver.com/live/{channelID} |
| CHZZK | Video | https://chzzk.naver.com/video/{videoNo} |
| CHZZK | Clip | https://chzzk.naver.com/clips/{clipID} |
| NAVER TV | Live | https://tv.naver.com/l/{liveNo} |
| NAVER TV | VOD | https://tv.naver.com/v/{vodNo} |
| NAVER TV | Clip | https://tv.naver.com/h/{clipNo} |
| Shopping Live | Live | https://view.shoppinglive.naver.com/lives/{broadcastID} |
| Shopping Live | Short Clip | https://view.shoppinglive.naver.com/shortclips/{shortClipID} |
| WEBTOON | Cuts | https://comic.naver.com/cuts/v?id={cutsID} |
| SBS | Live | https://www.sbs.co.kr/live/{channelID} |
| SBS | AllVOD | https://allvod.sbs.co.kr/watch/{group}/{programID}/{mediaID} |
| SBS | Program | https://programs.sbs.co.kr/{section}/{programCode}/{group}/{menuID}/{mediaID} |

### General Features
- Cross-platform support (Windows, macOS, Linux)
- Simple and intuitive user interface
- Always on top

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