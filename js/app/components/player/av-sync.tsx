export const QUIET_PROFILE = "audible";

export async function quietReceiver(
  mediaStream: MediaStream,
  playerEvent: (time: string, eventType: string, data: any) => void,
) {
  let audioTime = 0;
  let videoTime = 0;
  let baseline = 0;

  const diff = (a: number, b: number) => {
    if (audioTime === 0 || videoTime === 0) {
      return;
    }
    if (baseline === 0) {
      baseline = audioTime - videoTime;
      console.log("baseline", baseline);
    }
    console.log("diff", audioTime - videoTime - baseline);
    playerEvent(new Date().toISOString(), "av-sync", {
      diff: audioTime - videoTime - baseline,
    });
  };

  const gotVideo = (time: number) => {
    videoTime = time;
    // diff(audioTime, videoTime);
  };

  const gotAudio = (time: number) => {
    audioTime = time;
    diff(audioTime, videoTime);
  };

  const Quiet = await import("quietjs-bundle");
  Quiet.addReadyCallback(() => {
    const nav = navigator as unknown as any;
    // quiet doesn't let us pass in a mediaStream so we need to monkeypatch getusermedia
    const getUserMedia = nav.getUserMedia;
    nav.getUserMedia = async (constraints, cb) => {
      cb(mediaStream);
      // we're done, unmonkeypatch
      nav.getUserMedia = getUserMedia;
    };
    const quiet = Quiet.receiver({
      profile: QUIET_PROFILE,
      onReceive: (payload) => {
        try {
          const str = Quiet.ab2str(payload);
          const time = parseInt(str);
          gotAudio(time);
        } catch (e) {
          console.error("quiet receiver error", e);
        }
      },
      onCreate: () => {
        console.log("receiver created");
      },
      onCreateFail: (error) => {
        console.error("receiver failed to create", error);
      },
      onReceiveFail: (error) => {
        console.error("receiver failed to receive", error);
      },
      // onReceiverStatsUpdate: (stats) => {
      //   console.log("receiver stats", stats);
      // },
    });
  });

  const zxing = await import("@zxing/browser");
  const codeReader = new zxing.BrowserQRCodeReader();
  codeReader.decodeFromStream(mediaStream, undefined, (result, err) => {
    try {
      if (result) {
        const time = parseInt(result.getText());
        gotVideo(time);
      }
    } catch (e) {
      console.error("zxing error", e);
    }
  });
}
