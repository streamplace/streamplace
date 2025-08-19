import ReplayKit
import AVFoundation
import Foundation

class SampleHandler: RPBroadcastSampleHandler {

    private var socketConnection: SocketConnection?
    private var uploader: SampleUploader?
    private var isRecording = false

    override func broadcastStarted(withSetupInfo setupInfo: [String : NSObject]?) {
        // User has requested to start the broadcast. Setup info from the UI extension can be supplied but optional.
        print("Broadcast started with setup info: \(String(describing: setupInfo))")

        // Initialize socket connection for communication with main app
        socketConnection = SocketConnection()

        // Initialize uploader for handling sample data
        uploader = SampleUploader()

        // Set up communication with main app via Darwin notifications
        DarwinNotificationCenter.shared.postNotification(name: "BroadcastStarted")

        isRecording = true
    }

    override func broadcastPaused() {
        // User has requested to pause the broadcast. Samples will stop being delivered.
        print("Broadcast paused")

        isRecording = false
        DarwinNotificationCenter.shared.postNotification(name: "BroadcastPaused")
    }

    override func broadcastResumed() {
        // User has requested to resume the broadcast. Samples delivery will resume.
        print("Broadcast resumed")

        isRecording = true
        DarwinNotificationCenter.shared.postNotification(name: "BroadcastResumed")
    }

    override func broadcastFinished() {
        // User has requested to finish the broadcast.
        print("Broadcast finished")

        isRecording = false
        socketConnection?.disconnect()
        socketConnection = nil
        uploader = nil

        DarwinNotificationCenter.shared.postNotification(name: "BroadcastFinished")
    }

    override func processSampleBuffer(_ sampleBuffer: CMSampleBuffer, with sampleBufferType: RPSampleBufferType) {
        guard isRecording else {
            return
        }

        switch sampleBufferType {
        case RPSampleBufferType.video:
            // Handle video sample buffers (screen capture)
            processVideoSampleBuffer(sampleBuffer)

        case RPSampleBufferType.audioApp:
            // Handle audio sample buffers from the app
            processAudioSampleBuffer(sampleBuffer, type: .app)

        case RPSampleBufferType.audioMic:
            // Handle audio sample buffers from the microphone
            processAudioSampleBuffer(sampleBuffer, type: .mic)

        @unknown default:
            // Handle any future sample buffer types
            print("Unknown sample buffer type: \(sampleBufferType.rawValue)")
        }
    }

    private func processVideoSampleBuffer(_ sampleBuffer: CMSampleBuffer) {
        // Process video frames from screen capture
        guard let imageBuffer = CMSampleBufferGetImageBuffer(sampleBuffer) else {
            print("Failed to get image buffer from video sample")
            return
        }

        // Get presentation timestamp
        let presentationTimeStamp = CMSampleBufferGetPresentationTimeStamp(sampleBuffer)

        // Send video data to uploader
        uploader?.uploadVideoFrame(imageBuffer: imageBuffer, timestamp: presentationTimeStamp)

        // Optionally send via socket connection
        socketConnection?.sendVideoFrame(imageBuffer: imageBuffer, timestamp: presentationTimeStamp)
    }

    private func processAudioSampleBuffer(_ sampleBuffer: CMSampleBuffer, type: AudioType) {
        // Process audio samples
        guard let audioBufferList = getAudioBufferList(from: sampleBuffer) else {
            print("Failed to get audio buffer list from audio sample")
            return
        }

        let
