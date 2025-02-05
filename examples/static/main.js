'use strict';

// Publish Options
const pubSimulcastSelect = document.querySelector('#pubSimulcast');
const pubSpatialLayersSelect = document.querySelector('#pubSpatialLayers');
const pubScaleResolutionDownBySelect = document.querySelector('#pubScaleResolutionDownBy');
const pubResolutionMaintainCheckbox = document.querySelector('#pubResolutionMaintain');
const pubVideoVideo = document.querySelector('#pubVideo');
const pubAudioCanvas = document.querySelector('#pubAudio');
const pubVideoInfoDiv = document.querySelector('#pubVideoInfo');
const pubpcTracksButton = document.querySelector('#pubpcTracks');
// Subscribe Options
const subVideoVideo = document.querySelector('#subVideo');
const subAudioStubVideo = document.querySelector('#subAudioStub');
const subAudioCanvas = document.querySelector('#subAudio');
const subVideoInfoDiv = document.querySelector('#subVideoInfo');
const subProfileSelect = document.querySelector('#subProfile');
const subpcTracksButton = document.querySelector('#subpcTracks');
const subChangeLayerButton = document.querySelector('#subChangeLayer');
let sessionId = getRandomId(12);

// Global Variable
var pubStreamVisualizer = null;
var subStreamVisualizer = null;

function getParameterByName(name, url) {
    if (!url) url = window.location.href;
    name = name.replace(/[\[\]]/g, '\\$&');
    var regex = new RegExp('[?&]' + name + '(=([^&#]*)|&|#|$)'),
        results = regex.exec(url);
    if (!results) return null;
    if (!results[2]) return '';
    return decodeURIComponent(results[2].replace(/\+/g, ' '));
}


// Initialize
window.onload = () => {
    let t= getParameterByName("sessionId",null)
    if (t!==""){
        sessionId = t;
    }

    // Publish Options Initialize
    pubVideoInfoDiv.style.display = 'none';
    pubpcTracksButton.addEventListener('click', publish);
    // Subscribe Options Initialize
    subVideoInfoDiv.style.display = 'none';
    subpcTracksButton.addEventListener('click', subscribe);
    subChangeLayerButton.addEventListener('click', changeProfile);
    // Get support constraint
    console.log(navigator.mediaDevices.getSupportedConstraints());

}


function getRandomId(length) {
    let result = [];
    for (var i = 0; i < length; i++) {
        var ranNum = Math.ceil(Math.random() * 35);
        if (ranNum <= 25) {
            result.push(String.fromCharCode(97 + ranNum));
        } else {
            result.push(String.fromCharCode(48 + ranNum - 26));
        }
    }
    return result.join('');
}

async function publish() {
    let peerConnection = new RTCPeerConnection({
        bundlePolicy: 'max-bundle',
        rtcpMuxPolicy: 'require',
        iceTransportPolicy: 'all',
    })
    try {
        let videoConstraints = true;
        if (pubSimulcastSelect.value == 'rid-based') {
            videoConstraints = {
                width: {
                    exact: 1280,
                },
                height: {
                    exact: 640,
                },
                frameRate: {
                    max: 25,
                }
            };
        }
        var stream = await navigator.mediaDevices.getUserMedia({audio: true, video: videoConstraints});
        let audioPubTransceiver = peerConnection.addTransceiver(stream.getAudioTracks()[0], {direction: 'sendonly'});

        let videoEncodings = [];
        let scaleRes = parseFloat(pubScaleResolutionDownBySelect.value)
        if (pubSimulcastSelect.value == 'rid-based') {
            videoEncodings.push({rid: 'hi', active: true, maxBitrate: 900000});
            if (parseInt(pubSpatialLayersSelect.value) > 2) {
                videoEncodings.push({rid: 'mid', active: true, maxBitrate: 300000, scaleResolutionDownBy: scaleRes});
                scaleRes = scaleRes * scaleRes
            }
            if (parseInt(pubSpatialLayersSelect.value) >= 2) {
                videoEncodings.push({rid: 'lo', active: true, maxBitrate: 100000, scaleResolutionDownBy: scaleRes});
            }
        }
        let videoPubTransceiver = peerConnection.addTransceiver(stream.getVideoTracks()[0], {
            direction: 'sendonly',
            active: true,
            sendEncodings: videoEncodings
        });
        let offer = await peerConnection.createOffer();
        await peerConnection.setLocalDescription(offer);
        console.log(offer.sdp)
        fetch("/pub?sessionId=" + sessionId, {
            method: 'post',
            headers: {
                'Accept': 'application/json, text/plain, */*',
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(offer)
        })
            .then(res => res.json())
            .then(res => {
                peerConnection.setRemoteDescription(res)
                console.log(res.sdp)
                pubVideoVideo.srcObject = new MediaStream([videoPubTransceiver.sender.track]);
                pubVideoVideo.onresize = () => pubVideoInfoDiv.textContent = 'video dimensions: ' + pubVideoVideo.videoWidth + 'x' + pubVideoVideo.videoHeight;
                pubVideoInfoDiv.style.display = '';
                let ms = new MediaStream([audioPubTransceiver.sender.track]);
                pubStreamVisualizer = new StreamVisualizer(ms, pubAudioCanvas);
                pubStreamVisualizer.start();
            })
            .catch(alert)
        console.debug(`publish offer:\n%c${offer.sdp}`, 'color:cyan');
    } catch (e) {
        console.log(`publish error: ${e}`);
    }
}


function changeProfile() {
    fetch("/change?sessionId=" + sessionId + "&profile=" + subProfileSelect.value, {
        method: 'post',
        headers: {
            'Accept': 'application/json, text/plain, */*',
            'Content-Type': 'application/json'
        }
    }).then(res => {
        console.log("change result:", res);
    })
}

async function subscribe() {
    let peerConnection = new RTCPeerConnection();
    peerConnection.ontrack = (env) => {
        if (env.track.kind === 'video') {
            subVideoVideo.srcObject = new MediaStream([env.track]);
            subVideoVideo.onresize = () => subVideoInfoDiv.textContent = 'video dimensions: ' + subVideoVideo.videoWidth + 'x' + subVideoVideo.videoHeight;
            subVideoInfoDiv.style.display = '';
        } else if (env.track.kind === 'audio') {
            let ms = new MediaStream([env.track]);
            subAudioStubVideo.srcObject = ms
            subStreamVisualizer = new StreamVisualizer(ms, subAudioCanvas);
            subStreamVisualizer.start();
        }
    };
    fetch("/sub?sessionId=" + sessionId, {
        method: 'post',
        headers: {
            'Accept': 'application/json, text/plain, */*',
            'Content-Type': 'application/json'
        }
    }).then(res => res.json()).then(res => {
        peerConnection.setRemoteDescription(res);
        peerConnection.createAnswer().then(answer => {
            peerConnection.setLocalDescription(answer);
        });
    })
}