import './style.css';
import './app.css';

import {PlayStream, CloseStream, OpenFile, SetAlwaysOnTop} from '../wailsjs/go/main/App';
import {EventsOn, EventsEmit} from '../wailsjs/runtime/runtime';

let mediaSource, sourceBuffer;
let frameQueue = [];
let isAppending = false;
let isReconnecting = false;

const storageKeyURL = "playgo:ui:url";
const storageKeyAlwaysOnTop = "playgo:setting:alwaysOnTop";

const btnPlayGo = document.getElementById("btnPlayGo");
const btnReconnect = document.getElementById("btnReconnect");
const inputUrl = document.getElementById("inputUrl");
const elVideo = document.getElementById("elVideo");
const imgPoster = document.getElementById("imgPoster");
const btnMenu = document.getElementById("btnMenu");
const dropdownMenu = document.getElementById("dropdownMenu");
const menuOpenFile = document.getElementById("menuOpenFile");
const menuAlwaysOnTop = document.getElementById("menuAlwaysOnTop");

function initialize() {
    const lastURL = localStorage.getItem(storageKeyURL);
    if (lastURL) {
        inputUrl.value = lastURL;
    }

    const isAlwaysOnTop = localStorage.getItem(storageKeyAlwaysOnTop);
    if (isAlwaysOnTop === "true") {
        SetAlwaysOnTop(true);
        menuAlwaysOnTop.classList.add("checked");
    }
}

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
                inputUrl.value = filePath;
                window.OnPlayGo();
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

inputUrl.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
        event.preventDefault();
        window.OnPlayGo();
    }
});

elVideo.addEventListener("playing", () => {
    imgPoster.style.display = "none";
});

elVideo.addEventListener("error", (e) => {
    const error = elVideo.error;
    if (error) {
        console.error(`video error: code=${error.code}, message=${error.message}`, e);
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

    inputUrl.disabled = false;
    menuOpenFile.classList.remove("disabled");
    btnPlayGo.innerText = "PlayGo";
    btnReconnect.disabled = true;
}

window.OnPlayGo = function () {
    if (btnPlayGo.innerText !== "PlayGo") {
        CloseStream();
    } else {
        const url = inputUrl.value;
        if (!url) {
            return;
        }

        localStorage.setItem(storageKeyURL, url);
        btnPlayGo.innerText = "Cancel";
        inputUrl.disabled = true;
        menuOpenFile.classList.add("disabled");
        PlayStream(url).then(ok => {
            if (!ok) {
                btnPlayGo.innerText = "PlayGo";
                inputUrl.disabled = false;
                menuOpenFile.classList.remove("disabled");
            }
        });
    }
};

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
        window.OnPlayGo();
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
