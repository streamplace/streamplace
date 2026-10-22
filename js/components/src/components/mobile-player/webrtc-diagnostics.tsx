import { useEffect, useState } from "react";

export interface WebRTCDiagnostics {
  done: boolean;
  browserSupport: boolean;
  rtcPeerConnection: boolean;
  rtcSessionDescription: boolean;
  getUserMedia: boolean;
  getDisplayMedia: boolean;
  isHwH264Supported: boolean;
  isAV1Supported: boolean;
  errors: string[];
  warnings: string[];
}

export function useWebRTCDiagnostics(): WebRTCDiagnostics {
  const [diagnostics, setDiagnostics] = useState<WebRTCDiagnostics>({
    done: false,
    browserSupport: false,
    rtcPeerConnection: false,
    rtcSessionDescription: false,
    getUserMedia: false,
    getDisplayMedia: false,
    isHwH264Supported: false,
    isAV1Supported: false,
    errors: [],
    warnings: [],
  });

  useEffect(() => {
    const errors: string[] = [];
    const warnings: string[] = [];

    // Check if we're in a browser environment
    if (typeof window === "undefined") {
      errors.push("Running in non-browser environment");
      setDiagnostics({
        done: false,
        browserSupport: false,
        rtcPeerConnection: false,
        rtcSessionDescription: false,
        getUserMedia: false,
        getDisplayMedia: false,
        isHwH264Supported: false,
        isAV1Supported: false,
        errors,
        warnings,
      });
      return;
    }

    // Check RTCPeerConnection support
    const rtcPeerConnection = !!(
      window.RTCPeerConnection ||
      (window as any).webkitRTCPeerConnection ||
      (window as any).mozRTCPeerConnection
    );

    if (!rtcPeerConnection) {
      errors.push("RTCPeerConnection is not supported");
    }

    // Check RTCSessionDescription support
    const rtcSessionDescription = !!(
      window.RTCSessionDescription ||
      (window as any).webkitRTCSessionDescription ||
      (window as any).mozRTCSessionDescription
    );

    if (!rtcSessionDescription) {
      errors.push("RTCSessionDescription is not supported");
    }

    // Check getUserMedia support
    const getUserMedia = !!(
      navigator.mediaDevices?.getUserMedia ||
      (navigator as any).getUserMedia ||
      (navigator as any).webkitGetUserMedia ||
      (navigator as any).mozGetUserMedia
    );

    if (!getUserMedia) {
      warnings.push(
        "getUserMedia is not supported - webcam features unavailable",
      );
    }

    // Check getDisplayMedia support
    const getDisplayMedia = !!navigator.mediaDevices?.getDisplayMedia;

    if (!getDisplayMedia) {
      warnings.push(
        "getDisplayMedia is not supported - screen sharing unavailable",
      );
    }

    // Check if running over HTTPS (required for some WebRTC features)
    if (
      location.protocol !== "https:" &&
      location.hostname !== "localhost" &&
      location.hostname !== "127.0.0.1"
    ) {
      warnings.push("WebRTC features may be limited over HTTP connections");
    }

    // Check browser-specific issues
    const userAgent = navigator.userAgent.toLowerCase();
    if (userAgent.includes("safari") && !userAgent.includes("chrome")) {
      warnings.push("Safari may have limited WebRTC codec support");
    }

    const browserSupport = rtcPeerConnection && rtcSessionDescription;

    // Check video codec support asynchronously
    if (rtcPeerConnection) {
      videoCodecSupport()
        .catch((error) => {
          console.warn("Failed to check video codec support:", error);
          return { h264: false, av1: false };
        })
        .then(({ h264, av1 }) => {
          // The node sends AV1 when the stream has it and the viewer offers
          // it, else H.264.
          if (!h264) {
            warnings.push(
              `H.264 is not supported${av1 ? ", so only streams with AV1 play" : ""}\n In Firefox, try enabling 'media.webrtc.hw.h264.enabled' in about:config`,
            );
          }
          setDiagnostics({
            done: true,
            browserSupport,
            rtcPeerConnection,
            rtcSessionDescription,
            getUserMedia,
            getDisplayMedia,
            isHwH264Supported: h264,
            isAV1Supported: av1,
            errors,
            warnings,
          });
        });
    } else {
      setDiagnostics({
        done: true,
        browserSupport,
        rtcPeerConnection,
        rtcSessionDescription,
        getUserMedia,
        getDisplayMedia,
        isHwH264Supported: false,
        isAV1Supported: false,
        errors,
        warnings,
      });
    }
  }, []);

  return diagnostics;
}

// The video codecs this browser offers to receive over WebRTC.
async function videoCodecSupport(): Promise<{ h264: boolean; av1: boolean }> {
  const pc = new RTCPeerConnection();
  try {
    const { sdp = "" } = await pc.createOffer({ offerToReceiveVideo: true });
    return {
      h264: /rtpmap:\d+ H264\//.test(sdp),
      av1: /rtpmap:\d+ AV1/.test(sdp),
    };
  } finally {
    pc.close();
  }
}

export async function logWebRTCDiagnostics() {
  console.group("WebRTC Diagnostics");

  // Log browser support
  console.log("RTCPeerConnection:", !!window.RTCPeerConnection);
  console.log("RTCSessionDescription:", !!window.RTCSessionDescription);
  console.log("getUserMedia:", !!navigator.mediaDevices?.getUserMedia);
  console.log("getDisplayMedia:", !!navigator.mediaDevices?.getDisplayMedia);

  // Log browser info
  console.log("User Agent:", navigator.userAgent);
  console.log("Protocol:", location.protocol);
  console.log("Host:", location.hostname);
  console.groupEnd();
  if (window.RTCPeerConnection) {
    console.group("WebRTC Peer Connection Test");
    try {
      const { h264, av1 } = await videoCodecSupport();
      console.log("RTCPeerConnection creation: ✓ Success");
      console.log("H.264 support:", h264 ? "✓ Supported" : "✗ Not supported");
      console.log("AV1 support:", av1 ? "✓ Supported" : "✗ Not supported");
    } catch (error) {
      console.error("Video codec check failed:", error);
    }
  }
  console.groupEnd();
}
