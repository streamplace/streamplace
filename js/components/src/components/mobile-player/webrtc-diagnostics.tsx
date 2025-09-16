import { useEffect, useState } from "react";

export interface WebRTCDiagnostics {
  done: boolean;
  browserSupport: boolean;
  rtcPeerConnection: boolean;
  rtcSessionDescription: boolean;
  getUserMedia: boolean;
  getDisplayMedia: boolean;
  isHwH264Supported: boolean;
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
    errors: [],
    warnings: [],
  });

  useEffect(() => {
    const errors: string[] = [];
    const warnings: string[] = [];

    const checkH264Support = async (): Promise<boolean> => {
      try {
        const pc = new RTCPeerConnection();
        const offer = await pc.createOffer();
        pc.close();

        if (offer.sdp) {
          const h264Match = offer.sdp.search(/rtpmap:([0-9]+) H264/g);
          return h264Match !== -1;
        }
        return false;
      } catch (error) {
        console.warn("Failed to check H.264 support:", error);
        return false;
      }
    };

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

    // Check H.264 support asynchronously
    if (rtcPeerConnection) {
      checkH264Support().then((isHwH264Supported) => {
        if (!isHwH264Supported) {
          warnings.push(
            "H.264 hardware acceleration is not supported\n In Firefox, try enabling 'media.webrtc.hw.h264.enabled' in about:config",
          );
        }
        setDiagnostics({
          done: true,
          browserSupport,
          rtcPeerConnection,
          rtcSessionDescription,
          getUserMedia,
          getDisplayMedia,
          isHwH264Supported,
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
        errors,
        warnings,
      });
    }
  }, []);

  return diagnostics;
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
    try {
      const pc = new RTCPeerConnection();
      // Check H.264 support
      try {
        const offer = await pc.createOffer({ offerToReceiveVideo: true });
        const isHwH264Supported = offer.sdp
          ? offer.sdp.search(/rtpmap:([0-9]+) H264/g) !== -1
          : false;
        console.group("WebRTC Peer Connection Test");
        console.log("RTCPeerConnection creation: ✓ Success");
        console.log(
          "H.264 support:",
          isHwH264Supported ? "✓ Supported" : "✗ Not supported",
        );
      } catch (error) {
        console.group("WebRTC Peer Connection Test");
        console.error("H.264 check failed:", error);
      }

      pc.close();
    } catch (error) {
      console.group("WebRTC Peer Connection Test");
      console.error("RTCPeerConnection creation: ✗ Failed", error);
    }
  }
  console.groupEnd();
}
