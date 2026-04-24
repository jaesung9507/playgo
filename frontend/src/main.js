import './style.css';
import './app.css';

import EarthIcon from '~icons/mdi/earth';
import LockIcon from '~icons/mdi/lock';
import LockOffIcon from '~icons/mdi/lock-off';

import {PlayStream, CloseStream, OpenFile, SetAlwaysOnTop, MsgBox, Quit} from '../wailsjs/go/main/App';
import {EventsOn, EventsEmit} from '../wailsjs/runtime/runtime';

let mediaSource, sourceBuffer;
let frameQueue = [];
let isAppending = false;
let isReconnecting = false;

const storageKeyURL = "playgo:ui:url";
const storageKeyAlwaysOnTop = "playgo:setting:alwaysOnTop";

const btnPlayGo = document.getElementById("btnPlayGo");
const btnReconnect = document.getElementById("btnReconnect");
const inputURL = document.getElementById("inputURL");
const iconURL = document.getElementById("iconURL");
const elVideo = document.getElementById("elVideo");
const imgPoster = document.getElementById("imgPoster");
const btnMenu = document.getElementById("btnMenu");
const dropdownMenu = document.getElementById("dropdownMenu");
const menuOpenFile = document.getElementById("menuOpenFile");
const menuAlwaysOnTop = document.getElementById("menuAlwaysOnTop");
const menuQuit = document.getElementById("menuQuit");

function setURLIcon(svg, title = "", color = "") {
    iconURL.innerHTML = svg;
    iconURL.title = title;
    iconURL.style.color = color;
}

function initialize() {
    const lastURL = localStorage.getItem(storageKeyURL);
    if (lastURL) {
        inputURL.value = lastURL;
    }

    const isAlwaysOnTop = localStorage.getItem(storageKeyAlwaysOnTop);
    if (isAlwaysOnTop === "true") {
        SetAlwaysOnTop(true);
        menuAlwaysOnTop.classList.add("checked");
    }

    setURLIcon(EarthIcon);
}

function onPlayGo() {
    if (btnPlayGo.innerText !== "PlayGo") {
        CloseStream();
    } else {
        const url = inputURL.value;
        if (!url) {
            return;
        }

        localStorage.setItem(storageKeyURL, url);
        btnPlayGo.innerText = "Cancel";
        inputURL.disabled = true;
        menuOpenFile.classList.add("disabled");
        PlayStream(url).then(ok => {
            if (!ok) {
                btnPlayGo.innerText = "PlayGo";
                inputURL.disabled = false;
                menuOpenFile.classList.remove("disabled");
            }
        });
    }
}

btnPlayGo.addEventListener("click", onPlayGo);

btnMenu.addEventListener("click", () => {
    dropdownMenu.classList.toggle("show");
    btnMenu.classList.toggle("active");
});

btnReconnect.addEventListener("click", () => {
    isReconnecting = true;
    CloseStream();
});

window.addEventListener("click", (event) => {
    if (!event.target.closest(".dropdown") || event.target.closest("a:not(.disabled)")) {
        if (dropdownMenu.classList.contains("show")) {
            dropdownMenu.classList.remove("show");
            btnMenu.classList.remove("active");
        }
    }
});

menuOpenFile.addEventListener("click", () => {
    if (!menuOpenFile.classList.contains("disabled")) {
        OpenFile().then(filePath => {
            if (filePath) {
                inputURL.value = filePath;
                onPlayGo();
            }
        });
    }
});

menuAlwaysOnTop.addEventListener("click", () => {
    const isAlwaysOnTop = !menuAlwaysOnTop.classList.contains("checked");
    SetAlwaysOnTop(isAlwaysOnTop);
    menuAlwaysOnTop.classList.toggle("checked", isAlwaysOnTop);
    localStorage.setItem(storageKeyAlwaysOnTop, isAlwaysOnTop);
});

menuQuit.addEventListener("click", Quit);

inputURL.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
        event.preventDefault();
        onPlayGo();
    }
});

elVideo.addEventListener("playing", () => {
    imgPoster.style.display = "none";
});

elVideo.addEventListener("error", (e) => {
    const error = elVideo.error;
    if (error) {
        console.error(`video error: code=${error.code}, message=${error.message}`, e);
        MsgBox(error.message);
        CloseStream();
    }
});

function resetVideo() {
    imgPoster.style.display = "block";
    frameQueue = [];
    isAppending = false;
    mediaSource = null;
    sourceBuffer = null;

    if (elVideo.src) window.URL.revokeObjectURL(elVideo.src);

    elVideo.pause();
    elVideo.removeAttribute("src");
    elVideo.currentTime = 0;
    elVideo.load();

    inputURL.disabled = false;
    menuOpenFile.classList.remove("disabled");
    btnPlayGo.innerText = "PlayGo";
    btnReconnect.disabled = true;

    setURLIcon(EarthIcon);
}

EventsOn("OnSecureInfo", function (secured, trusted, info) {
    if (secured) {
        if (trusted) {
            setURLIcon(LockIcon, "Connection secure\n\n" +
                Object.entries(info).map(([k, v]) => `${k}: ${v}`).join("\n"));
        } else {
            setURLIcon(LockIcon, "Connection not verified\n\n" +
                Object.entries(info).map(([k, v]) => `${k}: ${v}`).join("\n"), "firebrick");
        }
    } else {
        setURLIcon(LockOffIcon, "Connection not secure", "firebrick");
    }
});

EventsOn("OnInit", function (meta, init) {
    btnPlayGo.innerText = "Stop";
    btnReconnect.disabled = false;
    mediaSource = new MediaSource();
    elVideo.src = window.URL.createObjectURL(mediaSource);

    mediaSource.addEventListener("sourceopen", function () {
        if (mediaSource.sourceBuffers.length > 0) return;

        try {
            sourceBuffer = mediaSource.addSourceBuffer(`video/mp4; codecs="${meta}"`);
        } catch (e) {
            console.error("failed to add source buffer:", e);
            return;
        }

        sourceBuffer.mode = "segments";
        sourceBuffer.addEventListener("updateend", onUpdateEnd);
        sourceBuffer.addEventListener("error", (e) => console.error("source buffer error:", e));

        let initAppendDone = function() {
            sourceBuffer.removeEventListener("updateend", initAppendDone);
            EventsEmit("OnUpdateEnd");
            elVideo.play().catch(e => console.error("failed to play video:", e));
        };
        sourceBuffer.addEventListener("updateend", initAppendDone);
        pushBuffer(init);
    });
});

EventsOn("OnStreamStop", () => {
    console.log("OnStreamStop");
    resetVideo();
    if (isReconnecting) {
        isReconnecting = false;
        onPlayGo();
    }
});

EventsOn("OnFrame", function (frame) {
    pushBuffer(frame);
});

function pushBuffer(encoded) {
    const decoded = atob(encoded);
    const arr = new Uint8Array(decoded.length);
    for (let i = 0; i < decoded.length; i++) {
        arr[i] = decoded.charCodeAt(i);
    }
    frameQueue.push(arr.buffer);
    appendNextFrame();
}

function appendNextFrame() {
    if (isAppending || frameQueue.length === 0 || !sourceBuffer || sourceBuffer.updating) {
        return;
    }

    isAppending = true;
    const frame = frameQueue.shift();

    appendFrameData(frame);
}

function appendFrameData(frame) {
    try {
        sourceBuffer.appendBuffer(frame);
    } catch (e) {
        console.error("failed to append frame:", e);
        isAppending = false;
    }
}

function onUpdateEnd() {
    isAppending = false;
    if (elVideo.paused) {
        elVideo.play().catch(e => console.warn("failed to resume play:", e));
    }
    appendNextFrame();
}

initialize();
