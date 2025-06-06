// Browser compatibility checks for WebRTC
const checkWebRTCSupport = () => {
  if (typeof window === "undefined") {
    throw new Error("WebRTC is not available in non-browser environments");
  }

  if (!window.RTCPeerConnection) {
    throw new Error(
      "RTCPeerConnection is not supported in this browser. Please use a modern browser that supports WebRTC.",
    );
  }

  if (!window.RTCSessionDescription) {
    throw new Error(
      "RTCSessionDescription is not supported in this browser. Please use a modern browser that supports WebRTC.",
    );
  }
};

// Check support immediately
try {
  checkWebRTCSupport();
} catch (error) {
  console.error("WebRTC Compatibility Error:", error.message);
}

export const RTCPeerConnection = window.RTCPeerConnection;
export const RTCSessionDescription = window.RTCSessionDescription;
export const WebRTCMediaStream = window.MediaStream;
export const mediaDevices = navigator.mediaDevices;

// Export the compatibility checker for use in other components
export { checkWebRTCSupport };
