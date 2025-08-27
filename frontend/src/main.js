import './style.css';
import './app.css';

import {PlayStream, CloseStream} from '../wailsjs/go/main/App';
import {EventsOn, EventsEmit} from '../wailsjs/runtime/runtime';

let mediaSource, sourceBuffer;
let frameQueue = [];
let isAppending = false;

const btnPlayGo = document.getElementById("btnPlayGo");
const inputUrl = document.getElementById("inputUrl");
const elVideo = document.getElementById("elVideo");
elVideo.addEventListener("error", (e) => {
    const error = elVideo.error;
    if (error) {
        console.error(`video error: code=${error.code}, message=${error.message}`, e);
    }
});

function resetVideo() {
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
    btnPlayGo.innerText = "PlayGo";
}

window.OnPlayGo = function () {
    if (btnPlayGo.innerText === "Stop") {
        CloseStream();
    } else {
        const url = inputUrl.value;
        if (!url) {
            return;
        }

        btnPlayGo.disabled = true;
        inputUrl.disabled = true;
        PlayStream(url).then(ok => {
            btnPlayGo.disabled = false;
             if (!ok) inputUrl.disabled = false;
        });
    }
};

EventsOn("OnInit", function (meta, init) {
    btnPlayGo.innerText = "Stop";
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